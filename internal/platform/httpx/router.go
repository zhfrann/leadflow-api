package httpx

import (
	"context"
	"net/http"
	"time"
)

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type RouterConfig struct {
	Database         ReadinessChecker
	ReadinessTimeout time.Duration
	RegisterHandler  http.HandlerFunc
	LoginHandler     http.HandlerFunc
}

type statusResponse struct {
	Status string `json:"status"`
}

func NewHandler(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /ready", handleReady(cfg.Database, cfg.ReadinessTimeout))

	if cfg.RegisterHandler != nil {
		mux.HandleFunc("POST /v1/auth/register", cfg.RegisterHandler)
	}

	if cfg.LoginHandler != nil {
		mux.HandleFunc("POST /v1/auth/login", cfg.LoginHandler)
	}

	return RequestID(mux)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	_ = WriteJSON(
		w,
		http.StatusOK,
		statusResponse{
			Status: "ok",
		},
	)
}

func handleReady(database ReadinessChecker, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := database.Ping(ctx); err != nil {
			_ = WriteJSON(
				w,
				http.StatusServiceUnavailable,
				statusResponse{
					Status: "unavailable",
				},
			)
			return
		}

		_ = WriteJSON(
			w,
			http.StatusOK,
			statusResponse{
				Status: "ready",
			},
		)
	}
}
