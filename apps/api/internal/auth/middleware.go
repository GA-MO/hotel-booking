package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

type ctxKey int

const identityCtxKey ctxKey = iota

// WithIdentity attaches an Identity to ctx.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey, id)
}

// IdentityFrom retrieves the Identity from ctx (set by RequireAuth).
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityCtxKey).(Identity)
	return id, ok
}

// RequireAuth is middleware that verifies the Bearer access token and injects Identity.
func RequireAuth(j *JWT) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
				return
			}
			identity, err := j.ParseAccess(token)
			if err != nil {
				respond.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired token")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}
