package main

import (
	"bytes"
	"compress/gzip"
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
		Addr:     "localhost:6379", // TODO support not using localhost and a different port
		Password: "",               // TODO support password? maybe? secrets manager?
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
			log.Println("Failed to get message JSON:", err) // TODO send non-200
		}
		if setConfig.Compression == "gzip" {
			var buffer bytes.Buffer
			gz := gzip.NewWriter(&buffer)
			if _, err := gz.Write([]byte(data)); err != nil {
				log.Println("Failed to write to buffer", err)
			}
			if err := gz.Close(); err != nil {
				log.Println("Failed to close buffer:", err)
			}
			_, err = rdb.LPush(ctx, setConfig.QueueName, buffer.Bytes()).Result()
			if err != nil {
				log.Println("Failed to store message:", err) // TODO send non-200
			}
		} else {
			_, err = rdb.LPush(ctx, setConfig.QueueName, data).Result()
			if err != nil {
				log.Println("Failed to store message:", err) // TODO send non-200
			}
		}
		// TODO if message is too large send appropriate rejection
	})
	if err := router.Run(strings.Join([]string{":", strconv.Itoa(setConfig.ListenPort)}, "")); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
