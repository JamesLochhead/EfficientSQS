package common

import (
	"github.com/pelletier/go-toml/v2"
	"log"
	"os"
)

type Config struct {
	ListenPort            int    `toml:"port"`
	SqsMaximumMessageSize int    `toml:"sqsMaximumMessageSize"`
	SqsMinimumMessageSize int    `toml:"sqsMinimumMessageSize"`
	PollingMs             int    `toml:"pollingMs"`
	Mode                  string `toml:"mode"`
	RoutePattern          string `toml:"routePattern"`
	Compression           string `toml:"compression"`
	RedisQueueName        string `toml:"redisQueueName"`
	SqsQueueName          string `toml:"sqsQueueName"`
	SeparatingCharacters  string `toml:"separatingCharacters"`
}

func ProcessConfig() *Config {
	setConfig := Config{
		ListenPort:            8080,
		SqsMinimumMessageSize: 1,
		SqsMaximumMessageSize: 1048576,
		PollingMs:             100,
		Mode:                  "release",
		RoutePattern:          "/sqs",
		Compression:           "none",
		RedisQueueName:        "queue_b1946ac92",
	}
	b, err := os.ReadFile("../config.toml")
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
	if setConfig.SqsQueueName == "" {
		log.Fatalf("config: sqsQueueName must be set.")
	}
	if setConfig.SeparatingCharacters == "" {
		log.Fatalf("config: separatingCharacters must be set.")
	}
	return &setConfig
}
