package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"strconv"
	"strings"
)

type config struct {
	ListenPort int `toml:"port"`
	// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
	// The minimum message size is 1 byte (1 character). The maximum is 1,048,576 bytes (1 MiB).
	SqsMaximumMessageSize int    `toml:"sqsMaximumMessageSize"`
	SqsMinimumMessageSize int    `toml:"sqsMinimumMessageSize"`
	PollingMs             int    `toml:"pollingMs"`
	Mode                  string `toml:"mode"`
	RoutePattern          string `toml:"routePattern"`
	Compression           string `toml:"compression"`
	QueueName             string `toml:"queueName"`
}

func processConfig() *config {
	setConfig := config{
		ListenPort:            8080,
		SqsMinimumMessageSize: 1,
		SqsMaximumMessageSize: 1048576,
		PollingMs:             100,
		Mode:                  "release",
		RoutePattern:          "/sqs",
		Compression:           "gzip",
		QueueName:             "queue_b1946ac92",
	}
	b, err := os.ReadFile("config.toml")
	if err != nil {
		log.Fatalf("Failed to read config.toml: %v", err)
	}
	err = toml.Unmarshal(b, &setConfig)
	if err != nil {
		log.Fatalf("Failed to unmarshal config.toml: %v", err)
	}
	if setConfig.Compression != "gzip" && setConfig.Compression != "none" {
		log.Fatalf("config.toml: compression must be 'gzip' or 'none'.")
	}
	if setConfig.PollingMs < 50 || setConfig.PollingMs > 10000 {
		log.Fatalf("config: pollingMs must be greater than 50 and less than 10000.")
	}
	if setConfig.Mode != "debug" && setConfig.Mode != "release" {
		log.Fatalf("config: mode must be 'debug' or 'release'.")
	}
	return &setConfig
}

func main() {
	ctx := context.Background()
	setConfig := processConfig()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // TODO support not using localhost
		Password: "",               // TODO support password? maybe?
		DB:       0,
		Protocol: 2,
	})
	if setConfig.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.POST("/sqs", func(c *gin.Context) { // TODO pattern
		data, err := c.GetRawData()
		if err != nil {
			log.Println("Failed to get message JSON")
		}
		_, err = rdb.LPush(ctx, setConfig.QueueName, data).Result()
		if err != nil {
			log.Println("Failed to store message")
		}
	})
	if err := router.Run(strings.Join([]string{":", strconv.Itoa(setConfig.ListenPort)}, "")); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
