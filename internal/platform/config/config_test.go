package config_test

import (
	"testing"
	"time"

	"github.com/zhfrann/leadflow-api/internal/platform/config"
)

func TestLoadUsesDefaultValues(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDRESS", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Environment != "development" {
		t.Errorf(
			"expected environment development, got %s",
			cfg.Environment,
		)
	}

	if cfg.HTTPAddress != ":8080" {
		t.Errorf(
			"expected HTTP address :8080, got %s",
			cfg.HTTPAddress,
		)
	}

	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf(
			"expected shutdown timeout 10s, got %s",
			cfg.ShutdownTimeout,
		)
	}

	if cfg.LogLevel != "info" {
		t.Errorf(
			"expected log level info, got %s",
			cfg.LogLevel,
		)
	}
}

func TestLoadReadsEnvironmentVariables(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDRESS", ":9000")
	t.Setenv("SHUTDOWN_TIMEOUT", "20s")
	t.Setenv("LOG_LEVEL", "warn")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Environment != "production" {
		t.Errorf(
			"expected environment production, got %s",
			cfg.Environment,
		)
	}

	if cfg.HTTPAddress != ":9000" {
		t.Errorf(
			"expected HTTP address :9000, got %s",
			cfg.HTTPAddress,
		)
	}

	if cfg.ShutdownTimeout != 20*time.Second {
		t.Errorf(
			"expected shutdown timeout 20s, got %s",
			cfg.ShutdownTimeout,
		)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf(
			"expected log level warn, got %s",
			cfg.LogLevel,
		)
	}
}

func TestLoadRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "invalid-duration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "staging")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
