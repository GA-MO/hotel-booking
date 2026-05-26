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
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

// Error writes a JSON error envelope. If the response already carries an
// X-Request-Id header (set by the server's access-log middleware), the same
// id is echoed in the body so clients can include it in support tickets and
// developers can grep server logs without parsing headers separately.
func Error(w http.ResponseWriter, status int, code, message string) {
	resp := errorResponse{Error: ErrorBody{Code: code, Message: message}}
	if rid := w.Header().Get("X-Request-Id"); rid != "" {
		resp.RequestID = rid
	}
	Body(w, status, resp)
}
