package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

// errorEnvelope mirrors respond.Error's wire format for assertions.
type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErrorEnvelope(t *testing.T, body []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nbody=%s", err, string(body))
	}
	return env
}

func TestRequireAuthMissingHeader(t *testing.T) {
	j := NewJWT("secret", time.Minute, time.Hour)
	called := false
	h := RequireAuth(j)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if called {
		t.Fatal("wrapped handler should NOT have been called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want %d", rr.Code, http.StatusUnauthorized)
	}
	env := decodeErrorEnvelope(t, rr.Body.Bytes())
	if env.Error.Code != "UNAUTHORIZED" {
		t.Errorf("error code: got %q want %q", env.Error.Code, "UNAUTHORIZED")
	}
}

func TestRequireAuthMalformedHeader(t *testing.T) {
	j := NewJWT("secret", time.Minute, time.Hour)
	cases := []struct {
		name, header string
	}{
		{"no Bearer prefix", "Token abc.def.ghi"},
		{"lowercase bearer", "bearer abc.def.ghi"}, // case-sensitive prefix check
		{"empty token after Bearer", "Bearer "},
		{"only whitespace after Bearer", "Bearer    "},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := RequireAuth(j)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.Header.Set("Authorization", tc.header)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if called {
				t.Error("wrapped handler should NOT have been called")
			}
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status: got %d want %d", rr.Code, http.StatusUnauthorized)
			}
			env := decodeErrorEnvelope(t, rr.Body.Bytes())
			if env.Error.Code != "UNAUTHORIZED" {
				t.Errorf("error code: got %q want %q", env.Error.Code, "UNAUTHORIZED")
			}
		})
	}
}

func TestRequireAuthInvalidJWT(t *testing.T) {
	j := NewJWT("secret", time.Minute, time.Hour)
	called := false
	h := RequireAuth(j)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer not.a.realjwt")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if called {
		t.Fatal("wrapped handler should NOT have been called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want %d", rr.Code, http.StatusUnauthorized)
	}
	env := decodeErrorEnvelope(t, rr.Body.Bytes())
	if env.Error.Code != "INVALID_TOKEN" {
		t.Errorf("error code: got %q want %q", env.Error.Code, "INVALID_TOKEN")
	}
}

func TestRequireAuthValidJWT(t *testing.T) {
	j := NewJWT("secret", time.Minute, time.Hour)
	want := Identity{
		UserID:    uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		AccountID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		Role:      "manager",
	}
	tok, err := j.SignAccess(want)
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}

	var (
		called     bool
		gotID      Identity
		gotPresent bool
	)
	h := RequireAuth(j)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotID, gotPresent = IdentityFrom(r.Context())
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("wrapped handler was not called")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status: got %d want %d (teapot sentinel)", rr.Code, http.StatusTeapot)
	}
	if !gotPresent {
		t.Fatal("IdentityFrom returned ok=false; expected identity in context")
	}
	if gotID != want {
		t.Errorf("identity: got %+v want %+v", gotID, want)
	}
}

func TestIdentityFromAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if id, ok := IdentityFrom(req.Context()); ok {
		t.Errorf("expected ok=false on bare context; got id=%+v", id)
	}
}
