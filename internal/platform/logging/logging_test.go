package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zhfrann/leadflow-api/internal/platform/logging"
)

func TestNewWithWriterCreatesTextLoggerForDevelopment(t *testing.T) {
	var output bytes.Buffer

	logger, err := logging.NewWithWriter(logging.Config{
		Environment: "development",
		Level:       "info",
		Service:     "leadflow-api",
	}, &output)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logger.Info("application started")

	result := output.String()

	if !strings.Contains(result, "application started") {
		t.Errorf("expected log message, got %q", result)
	}

	if !strings.Contains(result, "service=leadflow-api") {
		t.Errorf("expected service field, got %q", result)
	}

	if !strings.Contains(result, "environment=development") {
		t.Errorf("expected environment field, got %q", result)
	}
}

func TestNewWithWriterCreatesJSONLoggerForProduction(t *testing.T) {
	var output bytes.Buffer

	logger, err := logging.NewWithWriter(logging.Config{
		Environment: "production",
		Level:       "info",
		Service:     "leadflow-api",
	}, &output)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logger.Info("application started")

	result := output.String()

	if !strings.Contains(result, `"msg":"application started"`) {
		t.Errorf("expected JSON log message, got %q", result)
	}

	if !strings.Contains(result, `"service":"leadflow-api"`) {
		t.Errorf("expected service field, got %q", result)
	}
}

func TestLoggerFiltersMessagesBelowConfiguredLevel(t *testing.T) {
	var output bytes.Buffer

	logger, err := logging.NewWithWriter(logging.Config{
		Environment: "development",
		Level:       "warn",
		Service:     "leadflow-api",
	}, &output)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logger.Info("information")
	logger.Warn("warning")

	result := output.String()

	if strings.Contains(result, "information") {
		t.Errorf("did not expect info log, got %q", result)
	}

	if !strings.Contains(result, "warning") {
		t.Errorf("expected warning log, got %q", result)
	}
}

func TestNewRejectsUnsupportedLogLevel(t *testing.T) {
	var output bytes.Buffer

	_, err := logging.NewWithWriter(logging.Config{
		Environment: "development",
		Level:       "trace",
		Service:     "leadflow-api",
	}, &output)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
