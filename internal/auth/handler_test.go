package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhfrann/leadflow-api/internal/auth"
	"github.com/zhfrann/leadflow-api/internal/platform/httpx"
)

type stubRegistrar struct {
	input  auth.RegisterInput
	user   auth.User
	err    error
	called bool
}

func (s *stubRegistrar) Register(_ context.Context, input auth.RegisterInput) (auth.User, error) {
	s.called = true
	s.input = input

	return s.user, s.err
}

type readyDatabase struct{}

func (readyDatabase) Ping(_ context.Context) error {
	return nil
}

func newRegisterTestHandler(registrar auth.Registrar) http.Handler {
	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	authHandler := auth.NewHandler(
		registrar,
		logger,
	)

	return httpx.NewHandler(httpx.RouterConfig{
		Database:         readyDatabase{},
		ReadinessTimeout: time.Second,
		RegisterHandler:  authHandler.Register,
	})
}

func TestRegisterEndpoint(t *testing.T) {
	createdAt := time.Date(
		2026,
		time.July,
		15,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	registrar := &stubRegistrar{
		user: auth.User{
			ID:        1,
			Email:     "user@example.com",
			CreatedAt: createdAt,
		},
	}

	handler := newRegisterTestHandler(registrar)

	body := bytes.NewBufferString(`{
		"email": "User@Example.com",
		"password": "very-secure-password"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		body,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			recorder.Code,
		)
	}

	if !registrar.called {
		t.Fatal("expected register service to be called")
	}

	if registrar.input.Email != "User@Example.com" {
		t.Errorf(
			"expected request email, got %q",
			registrar.input.Email,
		)
	}

	if registrar.input.Password !=
		"very-secure-password" {
		t.Errorf(
			"expected request password, got %q",
			registrar.input.Password,
		)
	}

	requestID := recorder.Header().Get(
		httpx.RequestIDHeader,
	)
	if requestID == "" {
		t.Fatal("expected request ID header")
	}

	var response struct {
		Data struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.ID != 1 {
		t.Errorf(
			"expected user ID 1, got %d",
			response.Data.ID,
		)
	}

	if response.Data.Email != "user@example.com" {
		t.Errorf(
			"unexpected email %q",
			response.Data.Email,
		)
	}
}

func TestRegisterRejectsUnknownField(t *testing.T) {
	registrar := &stubRegistrar{}
	handler := newRegisterTestHandler(registrar)

	body := bytes.NewBufferString(`{
		"email": "user@example.com",
		"password": "very-secure-password",
		"role": "admin"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		body,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}

	if registrar.called {
		t.Fatal("service must not be called")
	}
}

func TestRegisterReturnsConflictForDuplicateEmail(
	t *testing.T,
) {
	registrar := &stubRegistrar{
		err: auth.ErrEmailAlreadyExists,
	}

	handler := newRegisterTestHandler(registrar)

	body := bytes.NewBufferString(`{
		"email": "user@example.com",
		"password": "very-secure-password"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		body,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusConflict,
			recorder.Code,
		)
	}

	var response httpx.ErrorResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error.Code !=
		"EMAIL_ALREADY_REGISTERED" {
		t.Errorf(
			"unexpected error code %q",
			response.Error.Code,
		)
	}

	if response.Error.RequestID == "" {
		t.Fatal("expected request ID in response")
	}
}

func TestRegisterRejectsOversizedRequest(
	t *testing.T,
) {
	registrar := &stubRegistrar{}
	handler := newRegisterTestHandler(registrar)

	largePassword := strings.Repeat(
		"a",
		(1<<20)+100,
	)

	body := bytes.NewBufferString(
		`{"email":"user@example.com","password":"` +
			largePassword +
			`"}`,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		body,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code !=
		http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusRequestEntityTooLarge,
			recorder.Code,
		)
	}

	if registrar.called {
		t.Fatal("service must not be called")
	}
}

func TestRegisterReturnsInternalServerError(
	t *testing.T,
) {
	registrar := &stubRegistrar{
		err: errors.New("unexpected database error"),
	}

	handler := newRegisterTestHandler(registrar)

	body := bytes.NewBufferString(`{
		"email": "user@example.com",
		"password": "very-secure-password"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		body,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code !=
		http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			recorder.Code,
		)
	}
}
