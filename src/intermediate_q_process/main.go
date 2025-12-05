package main

import (
	"context"
	"github.com/JamesLochhead/EfficientSQS/src/common"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"log"
	"strconv"
	"strings"
)

func main() {
	ctx := context.Background()
	setConfig := common.ProcessConfig()
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
			log.Println("Failed to get message JSON") // TODO send non-200
		}
		_, err = rdb.LPush(ctx, setConfig.QueueName, data).Result()
		if err != nil {
			log.Println("Failed to store message") // TODO send non-200
		}
		// TODO if message is too large send appropriate rejection
	})
	if err := router.Run(strings.Join([]string{":", strconv.Itoa(setConfig.ListenPort)}, "")); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
