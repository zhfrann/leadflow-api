package httpx

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

func WriteJSON(w http.ResponseWriter, statusCode int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)

	return json.NewEncoder(w).Encode(payload)
}

func WriteError(
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	code string,
	message string,
) error {
	return WriteJSON(
		w,
		statusCode,
		ErrorResponse{
			Error: APIError{
				Code:      code,
				Message:   message,
				RequestID: RequestIDFromContext(r.Context()),
			},
		},
	)
}
