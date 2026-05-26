// Package httpx is the shared HTTP plumbing for domain handlers — role
// gating, URL-param parsing, JSON decoding. Every module under internal/
// previously copy-pasted these helpers; the copies drift the moment one
// gains an audit log or a stricter validator. Consolidating here gives one
// place to add cross-cutting behaviour.
//
// Auth-handler keeps its own decodeJSON to avoid a httpx→auth→httpx cycle
// (httpx imports auth for IdentityFrom).
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

// RequireRole returns a chi middleware that allows only authenticated callers
// whose role appears in allowed. Must be mounted AFTER auth.RequireAuth so the
// identity is in context. Returns 401 if no identity is set (defensive — this
// usually means a route wiring mistake), 403 if the role isn't allowed.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFrom(r.Context())
			if !ok {
				respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}
			if _, ok := allowedSet[identity.Role]; !ok {
				respond.Error(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ParseUUIDParam reads a chi URL param and parses it as a UUID. On failure
// writes a 400 and returns (uuid.Nil, false) — caller should return without
// writing further to w.
func ParseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

// DecodeJSON decodes r.Body into v. Rejects unknown fields per AGENTS.md
// convention. On failure writes a 400 and returns false.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return false
	}
	return true
}

// DecodeJSONOptional is like DecodeJSON but tolerates an empty body — used
// for endpoints where the body is optional (e.g. cancel with reason).
func DecodeJSONOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return false
	}
	return true
}
