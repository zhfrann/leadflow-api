package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultEnvironment     = "development"
	defaultHTTPAddress     = ":8080"
	defaultShutdownTimeout = 10 * time.Second
	defaultLogLevel        = "info"
)

type Config struct {
	Environment     string
	HTTPAddress     string
	ShutdownTimeout time.Duration
	LogLevel        string
}

func Load() (Config, error) {
	cfg := Config{
		Environment:     getEnv("APP_ENV", defaultEnvironment),
		HTTPAddress:     getEnv("HTTP_ADDRESS", defaultHTTPAddress),
		ShutdownTimeout: defaultShutdownTimeout,
		LogLevel:        getEnv("LOG_LEVEL", defaultLogLevel),
	}

	if value := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
		}

		cfg.ShutdownTimeout = timeout
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	switch c.Environment {
	case "development", "test", "production":
	default:
		return fmt.Errorf("APP_ENV must be development, test, or production")
	}

	if strings.TrimSpace(c.HTTPAddress) == "" {
		return fmt.Errorf("HTTP_ADDRESS must not be empty")
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be greater than zero")
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
