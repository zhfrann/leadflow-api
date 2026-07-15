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

type Authenticator interface {
	Login(ctx context.Context, input LoginInput) (LoginResult, error)
}

type LoginHandler struct {
	service Authenticator
	logger  *slog.Logger
}

func NewLoginHandler(service Authenticator, logger *slog.Logger) *LoginHandler {
	return &LoginHandler{
		service: service,
		logger:  logger,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Data loginResponseData `json:"data"`
}

type loginResponseData struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresAt   time.Time         `json:"expires_at"`
	User        loginResponseUser `json:"user"`
}

type loginResponseUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func (h *LoginHandler) Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxAuthRequestBodySize,
	)
	defer r.Body.Close()

	var request loginRequest

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

	result, err := h.service.Login(
		r.Context(),
		LoginInput{
			Email:    request.Email,
			Password: request.Password,
		},
	)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			h.writeError(
				w,
				r,
				http.StatusUnauthorized,
				"INVALID_CREDENTIALS",
				"Email or password is incorrect",
			)
			return
		}

		h.logger.Error(
			"login failed",
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
		return
	}

	err = httpx.WriteJSON(
		w,
		http.StatusOK,
		loginResponse{
			Data: loginResponseData{
				AccessToken: result.AccessToken,
				TokenType:   "Bearer",
				ExpiresAt:   result.ExpiresAt,
				User: loginResponseUser{
					ID:    result.User.ID,
					Email: result.User.Email,
				},
			},
		},
	)
	if err != nil {
		h.logger.Error(
			"write login response",
			"request_id",
			httpx.RequestIDFromContext(r.Context()),
			"error",
			err,
		)
	}
}

func (h *LoginHandler) writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
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

func (h *LoginHandler) writeError(
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
			"write login error response",
			"request_id",
			httpx.RequestIDFromContext(r.Context()),
			"error",
			err,
		)
	}
}
