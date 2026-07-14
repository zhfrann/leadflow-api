package main

import (
	"log"

	"github.com/zhfrann/leadflow-api/internal/platform/config"
	"github.com/zhfrann/leadflow-api/internal/platform/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	logger, err := logging.New(logging.Config{
		Environment: cfg.Environment,
		Level:       cfg.LogLevel,
		Service:     "leadflow-api",
	})
	if err != nil {
		log.Fatalf("initialize logger: %v", err)
	}

	logger.Info(
		"application starting",
		"address", cfg.HTTPAddress,
		"shutdown_timeout", cfg.ShutdownTimeout.String(),
	)
}
