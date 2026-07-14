package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zhfrann/leadflow-api/internal/platform/httpx"
)

type stubReadinessChecker struct {
	err error
}

func (s stubReadinessChecker) Ping(_ context.Context) error {
	return s.err
}

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handler := httpx.NewHandler(httpx.RouterConfig{
		Database:         stubReadinessChecker{},
		ReadinessTimeout: time.Second,
	})
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("expected status ok, got %q", response.Status)
	}

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf(
			"expected content type application/json, got %q",
			contentType,
		)
	}
}

func TestHealthEndpointRejectsUnsupportedMethod(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/health", nil)
	recorder := httptest.NewRecorder()

	handler := httpx.NewHandler(httpx.RouterConfig{
		Database:         stubReadinessChecker{},
		ReadinessTimeout: time.Second,
	})
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}
}

func TestReadyEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	recorder := httptest.NewRecorder()

	handler := httpx.NewHandler(httpx.RouterConfig{
		Database:         stubReadinessChecker{},
		ReadinessTimeout: time.Second,
	})

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestReadyEndpointWhenDatabaseUnavailable(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	recorder := httptest.NewRecorder()

	handler := httpx.NewHandler(httpx.RouterConfig{
		Database: stubReadinessChecker{
			err: errors.New("database unavailable"),
		},
		ReadinessTimeout: time.Second,
	})

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			recorder.Code,
		)
	}
}
