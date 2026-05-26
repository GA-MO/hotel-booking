package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"

	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
	jwt *JWT
}

func NewHandler(svc *Service, jwt *JWT) *Handler {
	return &Handler{svc: svc, jwt: jwt}
}

// Routes returns a chi router rooted at the auth prefix.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/signup", h.signup)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
	r.With(RequireAuth(h.jwt)).Get("/me", h.me)
	return r
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.Signup(r.Context(), req, requestMeta(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, resp)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.Login(r.Context(), req, requestMeta(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, resp)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.Refresh(r.Context(), req.RefreshToken, requestMeta(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, resp)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		writeAuthError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	identity, _ := IdentityFrom(r.Context())
	user, err := h.svc.Me(r.Context(), identity.UserID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, MeResponse{User: *user})
}

// ----- helpers -----

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func requestMeta(r *http.Request) RequestMeta {
	ip, _ := netip.ParseAddr(remoteIP(r))
	return RequestMeta{UserAgent: r.UserAgent(), IP: ip}
}

func remoteIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := indexComma(fwd); idx >= 0 {
			return trimSpace(fwd[:idx])
		}
		return trimSpace(fwd)
	}
	host := r.RemoteAddr
	if i := lastColon(host); i >= 0 {
		host = host[:i]
	}
	return host
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}
func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		respond.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
	case errors.Is(err, ErrEmailAlreadyExists):
		respond.Error(w, http.StatusConflict, "EMAIL_TAKEN", "this email is already registered")
	case errors.Is(err, ErrUserNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "user not found")
	case errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrSessionRevoked),
		errors.Is(err, ErrSessionExpired),
		errors.Is(err, ErrInvalidToken):
		respond.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired token")
	case errors.Is(err, ErrPasswordTooWeak):
		respond.Error(w, http.StatusBadRequest, "PASSWORD_WEAK", "password must be at least 8 characters and contain a letter and a digit")
	case errors.Is(err, ErrInvalidEmail):
		respond.Error(w, http.StatusBadRequest, "INVALID_EMAIL", "invalid email address")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
