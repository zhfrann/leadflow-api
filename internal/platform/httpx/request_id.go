package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

type contextKey string

const (
	requestIDContextKey contextKey = "request_id"
	RequestIDHeader                = "X-Request-ID"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		requestID := newRequestID()

		ctx := context.WithValue(
			r.Context(),
			requestIDContextKey,
			requestID,
		)

		w.Header().Set(RequestIDHeader, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	return requestID
}

func newRequestID() string {
	randomBytes := make([]byte, 12)

	if _, err := rand.Read(randomBytes); err == nil {
		return "req_" + hex.EncodeToString(randomBytes)
	}

	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
