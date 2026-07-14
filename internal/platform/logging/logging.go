package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

type Config struct {
	Environment string
	Level       string
	Service     string
}

func New(cfg Config) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, options)
	} else {
		handler = slog.NewTextHandler(os.Stdout, options)
	}

	logger := slog.New(handler).With(
		slog.String("service", cfg.Service),
		slog.String("environment", cfg.Environment),
	)

	return logger, nil
}

func NewWithWriter(cfg Config, writer io.Writer) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Environment == "production" {
		handler = slog.NewJSONHandler(writer, options)
	} else {
		handler = slog.NewTextHandler(writer, options)
	}

	logger := slog.New(handler).With(
		slog.String("service", cfg.Service),
		slog.String("environment", cfg.Environment),
	)

	return logger, nil
}

func parseLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}
