package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhfrann/leadflow-api/internal/auth"
	"github.com/zhfrann/leadflow-api/internal/platform/config"
	"github.com/zhfrann/leadflow-api/internal/platform/database"
	"github.com/zhfrann/leadflow-api/internal/platform/httpx"
	"github.com/zhfrann/leadflow-api/internal/platform/logging"
	"golang.org/x/crypto/bcrypt"
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

	postgresPool, err := database.NewPostgresPool(
		context.Background(),
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
	}
	defer postgresPool.Close()

	logger.Info(
		"PostgreSQL connection established",
		"max_connections", cfg.DatabaseMaxConns,
	)

	passwordHasher, err := auth.NewBcryptHasher(bcrypt.DefaultCost)
	if err != nil {
		logger.Error("initialize password hasher", "error", err)
		os.Exit(1)
	}

	jwtManager, err := auth.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTIssuer,
		cfg.JWTAudience,
		cfg.JWTAccessTTL,
	)
	if err != nil {
		logger.Error(
			"initialize JWT manager",
			"error", err,
		)
		os.Exit(1)
	}

	dummyPasswordHash, err := passwordHasher.Hash("leadflow-dummy-login-password")
	if err != nil {
		logger.Error(
			"create dummy password hash",
			"error", err,
		)
		os.Exit(1)
	}

	authRepository := auth.NewPostgresRepository(postgresPool)
	authService := auth.NewService(authRepository, passwordHasher)
	authHandler := auth.NewHandler(authService, logger)

	loginService := auth.NewLoginService(
		authRepository,
		passwordHasher,
		jwtManager,
		dummyPasswordHash,
	)
	loginHandler := auth.NewLoginHandler(loginService, logger)

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpx.NewHandler(httpx.RouterConfig{
			Database:         postgresPool,
			ReadinessTimeout: cfg.DatabaseHealthTimeout,
			RegisterHandler:  authHandler.Register,
			LoginHandler:     loginHandler.Login,
		}),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ErrorLog: slog.NewLogLogger(
			logger.Handler(),
			slog.LevelError,
		),
	}

	signalContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info(
			"HTTP server starting",
			"address", cfg.HTTPAddress,
		)

		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(
				"HTTP server stopped unexpectedly",
				"error", err,
			)
			os.Exit(1)
		}

	case <-signalContext.Done():
		logger.Info("shutdown signal received")

		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			cfg.ShutdownTimeout,
		)
		defer cancel()

		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error(
				"graceful shutdown failed",
				"error", err,
			)

			if closeErr := server.Close(); closeErr != nil {
				logger.Error(
					"force close HTTP server failed",
					"error", closeErr,
				)
			}

			os.Exit(1)
		}

		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(
				"HTTP server shutdown failed",
				"error", err,
			)
			os.Exit(1)
		}

		logger.Info("application stopped")
	}
}
