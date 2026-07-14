package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/zhfrann/leadflow-api/internal/platform/httpx"
)

const maxRegisterRequestBodySize = 1 << 20 // Shifting 1 bit to the left 20 times, equivalent to 1x2^20 = 1.048.576 bits = 1 MiB

type Registrar interface {
	Register(ctx context.Context, input RegisterInput) (User, error)
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	Data registerResponseData `json:"data"`
}

type registerResponseData struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Handler struct {
	service Registrar
	logger  *slog.Logger
}

func NewHandler(service Registrar, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxRegisterRequestBodySize,
	)
	defer r.Body.Close()

	var request registerRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		h.writeDecodeError(w, r, err)
		return
	}

	var additionalValue any

	if err := decoder.Decode(&additionalValue); !errors.Is(err, io.EOF) {
		h.writeError(
			w,
			r,
			http.StatusBadRequest,
			"INVALID_JSON",
			"Request body must contain one JSON object",
		)
		return
	}

	user, err := h.service.Register(
		r.Context(),
		RegisterInput{
			Email:    request.Email,
			Password: request.Password,
		},
	)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	response := registerResponse{
		Data: registerResponseData{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	}

	if err := httpx.WriteJSON(
		w,
		http.StatusCreated,
		response,
	); err != nil {
		h.logger.Error(
			"write register response",
			"request_id",
			httpx.RequestIDFromContext(r.Context()),
			"error",
			err,
		)
	}
}

func (h *Handler) writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var maxBytesError *http.MaxBytesError

	switch {
	case errors.As(err, &maxBytesError):
		h.writeError(
			w,
			r,
			http.StatusRequestEntityTooLarge,
			"REQUEST_TOO_LARGE",
			"Request body is too large",
		)

	case errors.Is(err, io.EOF):
		h.writeError(
			w,
			r,
			http.StatusBadRequest,
			"REQUEST_BODY_REQUIRED",
			"Request body is required",
		)

	default:
		h.writeError(
			w,
			r,
			http.StatusBadRequest,
			"INVALID_JSON",
			"Request body contains invalid JSON or unknown fields",
		)
	}
}

func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidEmail):
		h.writeError(
			w,
			r,
			http.StatusBadRequest,
			"INVALID_EMAIL",
			"Email is invalid",
		)

	case errors.Is(err, ErrPasswordTooShort):
		h.writeError(
			w,
			r,
			http.StatusBadRequest,
			"PASSWORD_TOO_SHORT",
			"Password must contain at least 12 characters",
		)

	case errors.Is(err, ErrPasswordTooLong):
		h.writeError(
			w,
			r,
			http.StatusBadRequest,
			"PASSWORD_TOO_LONG",
			"Password must not exceed 72 bytes",
		)

	case errors.Is(err, ErrEmailAlreadyExists):
		h.writeError(
			w,
			r,
			http.StatusConflict,
			"EMAIL_ALREADY_REGISTERED",
			"Email is already registered",
		)

	default:
		h.logger.Error(
			"register user failed",
			"request_id",
			httpx.RequestIDFromContext(r.Context()),
			"error",
			err,
		)

		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			"INTERNAL_SERVER_ERROR",
			"Internal server error",
		)
	}
}

func (h *Handler) writeError(
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	code string,
	message string,
) {
	if err := httpx.WriteError(
		w,
		r,
		statusCode,
		code,
		message,
	); err != nil {
		h.logger.Error(
			"write error response",
			"request_id",
			httpx.RequestIDFromContext(r.Context()),
			"error",
			err,
		)
	}
}
