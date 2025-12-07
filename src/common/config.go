package common

import (
	"github.com/pelletier/go-toml/v2"
	"log/slog"
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
	RedisHost             string `toml:"redisHost"`
	RedisPort             int    `toml:"redisPort"`
}

func ProcessConfig(logger *log/slog.Logger) *Config {
	setConfig := Config{
		ListenPort:            8080,
		SqsMinimumMessageSize: 1,
		SqsMaximumMessageSize: 1048576,
		PollingMs:             100,
		Mode:                  "release",
		RoutePattern:          "/sqs",
		Compression:           "none",
		RedisQueueName:        "queue_b1946ac92",
		RedisHost:             "localhost",
		RedisPort:             6379,
	}
	b, err := os.ReadFile("../efficient_sqs_config.toml") // TODO allow this to be elsewhere
	if err != nil {
		logger.Error("Failed to read efficient_sqs_config.toml", "error", err)
		os.Exit(1)
	}
	err = toml.Unmarshal(b, &setConfig)
	if err != nil {
		logger.Error("Failed to unmarshal config.toml", "error", err)
		os.Exit(1)
	}
	if setConfig.Compression != "gzip" && setConfig.Compression != "none" {
		logger.Error("config.toml: compression must be 'gzip' or 'none'.")
		os.Exit(1)
	}
	if setConfig.PollingMs < 50 || setConfig.PollingMs > 10000 {
		logger.Error("config: pollingMs must be greater than 50 and less than 10000.")
		os.Exit(1)
	}
	if setConfig.Mode != "debug" && setConfig.Mode != "release" {
		logger.Error("config: mode must be 'debug' or 'release'.")
		os.Exit(1)
	}
	if setConfig.SqsQueueName == "" {
		logger.Error("config: sqsQueueName must be set.")
		os.Exit(1)
	}
	if setConfig.SeparatingCharacters == "" {
		logger.Error("config: separatingCharacters must be set.")
		os.Exit(1)
	}
	return &setConfig
}
