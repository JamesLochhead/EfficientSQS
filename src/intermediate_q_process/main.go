package main

import (
	"context"
	"github.com/JamesLochhead/EfficientSQS/src/common"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()
	setConfig := common.ProcessConfig(logger)
	rdb := redis.NewClient(&redis.Options{
		Addr:     setConfig.RedisHost + ":" + strconv.Itoa(setConfig.RedisPort),
		Password: "", // TODO support password? maybe? secrets manager?
		DB:       0,
		Protocol: 2,
	})
	if setConfig.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.POST(setConfig.RoutePattern, func(c *gin.Context) { // TODO pattern
		data, err := c.GetRawData()
		if err != nil {
			logger.Error("Failed to get message JSON", "error", err)
			c.AbortWithStatusJSON(400, gin.H{
				"error": "invalid request body",
			})

		}
		if len(data) > setConfig.SqsMaximumMessageSize {
			c.JSON(413, gin.H{
				"error": "message too large; limit is 1MB",
			})
			return
		}
		if len(data) < setConfig.SqsMinimumMessageSize {
			c.JSON(400, gin.H{
				"error": "message too small",
			})
			return
		}
		if strings.Contains(string(data), setConfig.SeparatingCharacters) {
			c.JSON(400, gin.H{"error": "message contains invalid sequence"})
			return
		}
		_, err = rdb.LPush(ctx, setConfig.RedisQueueName, data).Result()
		if err != nil {
			logger.Error("Failed to store message", "error", err)
			c.AbortWithStatusJSON(500, gin.H{
				"error": "unable to store the message in Redis",
			})
		}
	})
	if err := router.Run(strings.Join([]string{":", strconv.Itoa(setConfig.ListenPort)}, "")); err != nil {
		logger.Error("Failed to run server", "error", err)
		os.Exit(1)
	}
}
