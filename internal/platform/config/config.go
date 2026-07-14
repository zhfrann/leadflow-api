package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEnvironment                  = "development"
	defaultHTTPAddress                  = ":8080"
	defaultHTTPReadHeaderTimeout        = 5 * time.Second
	defaultHTTPReadTimeout              = 10 * time.Second
	defaultHTTPWriteTimeout             = 30 * time.Second
	defaultHTTPIdleTimeout              = 60 * time.Second
	defaultShutdownTimeout              = 10 * time.Second
	defaultLogLevel                     = "info"
	defaultDatabaseConnectTimeout       = 5 * time.Second
	defaultDatabaseHealthTimeout        = 2 * time.Second
	defaultDatabaseMaxConns       int32 = 10
)

type Config struct {
	Environment            string
	HTTPAddress            string
	HTTPReadHeaderTimeout  time.Duration
	HTTPReadTimeout        time.Duration
	HTTPWriteTimeout       time.Duration
	HTTPIdleTimeout        time.Duration
	ShutdownTimeout        time.Duration
	LogLevel               string
	DatabaseURL            string
	DatabaseConnectTimeout time.Duration
	DatabaseHealthTimeout  time.Duration
	DatabaseMaxConns       int32
}

func Load() (Config, error) {
	readHeaderTimeout, err := getDurationEnv("HTTP_READ_HEADER_TIMEOUT", defaultHTTPReadHeaderTimeout)
	if err != nil {
		return Config{}, err
	}

	readTimeout, err := getDurationEnv("HTTP_READ_TIMEOUT", defaultHTTPReadTimeout)
	if err != nil {
		return Config{}, err
	}

	writeTimeout, err := getDurationEnv("HTTP_WRITE_TIMEOUT", defaultHTTPWriteTimeout)
	if err != nil {
		return Config{}, err
	}

	idleTimeout, err := getDurationEnv("HTTP_IDLE_TIMEOUT", defaultHTTPIdleTimeout)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := getDurationEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	databaseConnectTimeout, err := getDurationEnv("DATABASE_CONNECT_TIMEOUT", defaultDatabaseConnectTimeout)
	if err != nil {
		return Config{}, err
	}

	databaseHealthTimeout, err := getDurationEnv("DATABASE_HEALTH_TIMEOUT", defaultDatabaseHealthTimeout)
	if err != nil {
		return Config{}, err
	}

	databaseMaxConns, err := getInt32Env("DATABASE_MAX_CONNS", defaultDatabaseMaxConns)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:            getEnv("APP_ENV", defaultEnvironment),
		HTTPAddress:            getEnv("HTTP_ADDRESS", defaultHTTPAddress),
		HTTPReadHeaderTimeout:  readHeaderTimeout,
		HTTPReadTimeout:        readTimeout,
		HTTPWriteTimeout:       writeTimeout,
		HTTPIdleTimeout:        idleTimeout,
		ShutdownTimeout:        shutdownTimeout,
		LogLevel:               getEnv("LOG_LEVEL", defaultLogLevel),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DatabaseConnectTimeout: databaseConnectTimeout,
		DatabaseHealthTimeout:  databaseHealthTimeout,
		DatabaseMaxConns:       databaseMaxConns,
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

	if c.HTTPReadHeaderTimeout <= 0 {
		return fmt.Errorf("HTTP_READ_HEADER_TIMEOUT must be greater than zero")
	}

	if c.HTTPReadTimeout <= 0 {
		return fmt.Errorf("HTTP_READ_TIMEOUT must be greater than zero")
	}

	if c.HTTPWriteTimeout <= 0 {
		return fmt.Errorf("HTTP_WRITE_TIMEOUT must be greater than zero")
	}

	if c.HTTPIdleTimeout <= 0 {
		return fmt.Errorf("HTTP_IDLE_TIMEOUT must be greater than zero")
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be greater than zero")
	}

	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}

	if c.DatabaseConnectTimeout <= 0 {
		return fmt.Errorf("DATABASE_CONNECT_TIMEOUT must be greater than zero")
	}

	if c.DatabaseHealthTimeout <= 0 {
		return fmt.Errorf("DATABASE_HEALTH_TIMEOUT must be greater than zero")
	}

	if c.DatabaseMaxConns <= 0 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be greater than zero")
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

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return duration, nil
}

func getInt32Env(key string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return int32(parsed), nil
}
