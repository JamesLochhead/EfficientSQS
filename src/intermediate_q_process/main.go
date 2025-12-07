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
	router.POST("/sqs", func(c *gin.Context) { // TODO pattern
		data, err := c.GetRawData()
		if err != nil {
			logger.Error("Failed to get message JSON", "error", err) // TODO send non-200
		}
		_, err = rdb.LPush(ctx, setConfig.RedisQueueName, data).Result()
		if err != nil {
			logger.Error("Failed to store message", "error", err) // TODO send non-200
		}
		// TODO if message is too large send appropriate rejection
		// TODO if message is too small send appropriate rejection
		// TODO search message for separating characters and reject
	})
	if err := router.Run(strings.Join([]string{":", strconv.Itoa(setConfig.ListenPort)}, "")); err != nil {
		logger.Error("Failed to run server", "error", err)
		os.Exit(1)
	}
}
