package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Body writes a JSON response with the given status.
func Body(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("respond.Body encode", "err", err)
	}
}

// Empty writes a status-only response (no body).
func Empty(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error ErrorBody `json:"error"`
}

// Error writes a JSON error envelope.
func Error(w http.ResponseWriter, status int, code, message string) {
	Body(w, status, errorResponse{Error: ErrorBody{Code: code, Message: message}})
}
