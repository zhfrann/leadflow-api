package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhfrann/leadflow-api/internal/notification"
	"github.com/zhfrann/leadflow-api/internal/platform/config"
	"github.com/zhfrann/leadflow-api/internal/platform/database"
	"github.com/zhfrann/leadflow-api/internal/platform/logging"
	mailx "github.com/zhfrann/leadflow-api/internal/platform/mail"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	logger, err := logging.New(logging.Config{
		Environment: cfg.Environment,
		Level:       cfg.LogLevel,
		Service:     "leadflow-worker",
	})
	if err != nil {
		log.Fatalf("initialize logger: %v", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	postgresPool, err := database.NewPostgresPool(
		ctx,
		database.PostgresConfig{
			URL:            cfg.DatabaseURL,
			ConnectTimeout: cfg.DatabaseConnectTimeout,
			MaxConns:       cfg.DatabaseMaxConns,
		},
	)
	if err != nil {
		logger.Error(
			"initialize PostgreSQL",
			"error", err,
		)
		os.Exit(1)
	}
	defer postgresPool.Close()

	sender, err := mailx.NewSMTPSender(
		cfg.MailSMTPAddress,
		cfg.MailSMTPHost,
		cfg.MailFromName,
		cfg.MailFromAddress,
		cfg.MailTimeout,
	)
	if err != nil {
		logger.Error(
			"initialize SMTP sender",
			"error", err,
		)
		os.Exit(1)
	}

	templates, err := notification.NewTemplates()
	if err != nil {
		logger.Error(
			"initialize email templates",
			"error", err,
		)
		os.Exit(1)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	retryPolicy, err := notification.NewRetryPolicy(cfg.WorkerRetryDelays)
	if err != nil {
		logger.Error(
			"initialize email retry policy",
			"error", err,
		)
		os.Exit(1)
	}

	workerID := fmt.Sprintf(
		"%s-%d",
		hostname,
		os.Getpid(),
	)

	repository := notification.NewPostgresRepository(
		postgresPool,
	)

	worker := notification.NewWorker(
		repository,
		sender,
		templates,
		logger,
		workerID,
		cfg.WorkerPollInterval,
		retryPolicy,
		cfg.WorkerProcessingTimeout,
		cfg.WorkerRecoveryInterval,
	)

	if err := worker.Run(ctx); err != nil {
		logger.Error(
			"email worker stopped unexpectedly",
			"error", err,
		)
		os.Exit(1)
	}

	logger.Info("worker process stopped")
}
