package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/JamesLochhead/EfficientSQS/src/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// checkSqsExists checks whether an SQS queue with the given name
// exists by requesting its URL; it logs a fatal error and exits on
// failure, and returns true if the queue is found.
func checkSqsExists(
	sqsClient *sqs.Client,
	ctx context.Context,
	queueName string,
	logger *slog.Logger,
) bool {
	_, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		logger.Error("Couldn't get queue URL", "error", err)
		os.Exit(1)
	}
	return err == nil
}

// packBins consumes items from a Redis queue and groups them into
// bins such that each bin's total byte size stays below the SQS
// maximum message size. Items are appended to the first bin in which
// they fit, creating new bins as needed. The function returns maps of
// bin contents and bin sizes, or an error if Redis returns an
// unexpected failure.
func packBins(
	ctx context.Context,
	rdb *redis.Client,
	setConfig *common.Config,
) (map[int]string, error) {

	bins := make(map[int]string)
	binSizes := make(map[int]int)

	for {
		// Pop an item from Redis
		item, err := rdb.RPop(ctx, setConfig.RedisQueueName).Result()
		if err != nil {
			// Queue empty or Redis error
			if errors.Is(err, redis.Nil) {
				// normal "empty queue"
				return bins, nil
			}
			return bins, fmt.Errorf("redis pop error: %v", err)
		}

		itemLen := len(item)
		sepLen := len(setConfig.SeparatingCharacters)

		// Try to place the item in an existing or new bin
		i := 0
		for {
			currentSize := binSizes[i]
			newSize := currentSize + itemLen + sepLen

			if newSize < setConfig.SqsMaximumMessageSize {
				// Put item in bin i
				if len(bins[i]) == 0 {
					bins[i] = item
				} else {
					bins[i] = bins[i] +
						setConfig.SeparatingCharacters + item
				}
				binSizes[i] = newSize
				break
			}

			i++ // move to next bin
		}
	}
}

// chunkBins splits the input bins into chunks respecting both:
// - Max 10 messages per batch (SQS limit)
// - Max total batch size (configured SQS maximum)
func chunkBins(
	bins map[int]string,
	maxBatchSize int,
) []map[int]string {
	const maxMessagesPerBatch = 10

	var chunks []map[int]string
	current := make(map[int]string)
	currentSize := 0
	count := 0

	for k, v := range bins {
		messageSize := len(v)

		// Check if adding this message would exceed limits
		if count > 0 && (count >= maxMessagesPerBatch ||
			currentSize+messageSize > maxBatchSize) {
			// Finalize current chunk
			chunks = append(chunks, current)
			current = make(map[int]string)
			currentSize = 0
			count = 0
		}

		current[k] = v
		currentSize += messageSize
		count++
	}

	// Final partial chunk
	if count > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

// sendBatch sends a single bin group (up to 10 bins) to SQS as a
// SendMessageBatch call. No manual retries — AWS SDK handles retries
// automatically.
func sendBatch(
	ctx context.Context,
	client *sqs.Client,
	queueURL string,
	batch map[int]string,
	logger *slog.Logger,
) error {

	entries := make(
		[]sqstypes.SendMessageBatchRequestEntry,
		0,
		len(batch),
	)

	for k, v := range batch {
		entries = append(entries, sqstypes.SendMessageBatchRequestEntry{
			Id:          aws.String(fmt.Sprintf("bin-%d", k)),
			MessageBody: aws.String(v),
		})
	}

	input := &sqs.SendMessageBatchInput{
		QueueUrl: aws.String(queueURL),
		Entries:  entries,
	}

	resp, err := client.SendMessageBatch(ctx, input)
	if err != nil {
		return fmt.Errorf("batch send failed: %w", err)
	}

	// SQS does NOT retry partial failures — only retry logic is
	// SDK HTTP-level.
	if len(resp.Failed) > 0 {
		for _, f := range resp.Failed {
			logger.Error(
				"Partial batch failure",
				"id", *f.Id,
				"msg", *f.Message,
			)
		}
		return fmt.Errorf(
			"batch contains %d failed messages",
			len(resp.Failed),
		)
	}

	return nil
}

// drainAndFlush performs binpacking on all remaining items in Redis
// and sends them to SQS. This is called on graceful shutdown.
func drainAndFlush(
	ctx context.Context,
	rdb *redis.Client,
	sqsClient *sqs.Client,
	setConfig *common.Config,
	logger *slog.Logger,
) {
	logger.Info("Draining Redis and flushing to SQS")

	bins, err := packBins(ctx, rdb, setConfig)
	if err != nil {
		logger.Error("Failed to pack bins during shutdown", "error", err)
		return
	}

	if len(bins) == 0 {
		logger.Info("No messages to flush")
		return
	}

	chunks := chunkBins(bins, setConfig.SqsMaximumMessageSize)

	var wg sync.WaitGroup
	for _, batch := range chunks {
		batchCopy := batch

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sendBatch(
				ctx,
				sqsClient,
				setConfig.SqsQueueName,
				batchCopy,
				logger,
			)
			if err != nil {
				logger.Error("Batch FAILED during shutdown", "error", err)
			} else {
				logger.Info(
					"Batch sent successfully during shutdown",
				)
			}
		}()
	}

	wg.Wait()
	logger.Info("Drain complete", "bins_flushed", len(bins))
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()
	setConfig := common.ProcessConfig(logger)
	rdb := redis.NewClient(&redis.Options{
		Addr: setConfig.RedisHost + ":" +
			strconv.Itoa(setConfig.RedisPort),
		Password: "", // TODO support password? secrets manager?
		DB:       0,
		Protocol: 2,
	})
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("Failed to setup AWS client", "error", err)
		os.Exit(1)
	}
	sqsClient := sqs.NewFromConfig(sdkConfig)
	checkSqsExists(sqsClient, ctx, setConfig.SqsQueueName, logger)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Channel to signal the main loop to stop
	done := make(chan bool)

	// Run main processing loop in a goroutine
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				time.Sleep(
					time.Duration(setConfig.PollingMs) *
						time.Millisecond,
				)
				bins, err := packBins(ctx, rdb, setConfig)
				if err != nil {
					logger.Error("Failed to pop from Redis", "error", err)
				}
				chunks := chunkBins(
					bins,
					setConfig.SqsMaximumMessageSize,
				)

				var wg sync.WaitGroup
				for _, batch := range chunks {
					batchCopy := batch // avoid loop variable capture

					wg.Add(1)
					go func() {
						defer wg.Done()
						err := sendBatch(
							ctx,
							sqsClient,
							setConfig.SqsQueueName,
							batchCopy,
							logger,
						)
						if err != nil {
							logger.Error("Batch FAILED", "error", err)
						} else {
							logger.Info("Batch sent successfully")
						}
					}()
				}

				wg.Wait()
			}
		}
	}()

	// Wait for termination signal
	sig := <-sigChan
	logger.Info(
		"Received signal, initiating graceful shutdown",
		"signal", sig.String(),
	)

	// Stop the main loop
	close(done)

	// Drain remaining messages from Redis and flush to SQS
	drainAndFlush(ctx, rdb, sqsClient, setConfig, logger)

	logger.Info("Shutdown complete")
}
