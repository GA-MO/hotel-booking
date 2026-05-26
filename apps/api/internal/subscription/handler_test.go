package subscription

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

func TestRequireRole(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		identityRole string
		hasIdentity  bool
		allowed      []string
		wantStatus   int
		wantCode     string
	}{
		{"no identity", "", false, []string{"owner"}, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"role not allowed", "front_desk", true, []string{"owner"}, http.StatusForbidden, "FORBIDDEN"},
		{"owner allowed", "owner", true, []string{"owner"}, http.StatusOK, ""},
		{"manager not allowed (owner-only)", "manager", true, []string{"owner"}, http.StatusForbidden, "FORBIDDEN"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			handler := requireRole(c.allowed...)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.hasIdentity {
				req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{
					UserID:    uuid.New(),
					AccountID: uuid.New(),
					Role:      c.identityRole,
				}))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != c.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%s)", c.wantStatus, rec.Code, rec.Body.String())
			}
			if c.wantCode != "" {
				var env struct {
					Error respond.ErrorBody `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
					t.Fatalf("decode error envelope: %v", err)
				}
				if env.Error.Code != c.wantCode {
					t.Fatalf("error code: want %q, got %q", c.wantCode, env.Error.Code)
				}
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		var req UpdatePaymentMethodRequest
		r := httptest.NewRequest(http.MethodPost, "/",
			strings.NewReader(`{"provider":"stripe","method_id":"pm_x","last4":"4242","brand":"visa"}`))
		w := httptest.NewRecorder()
		if !decodeJSON(w, r, &req) {
			t.Fatalf("want ok, got false (body=%s)", w.Body.String())
		}
		if req.Provider != ProviderStripe || req.MethodID != "pm_x" {
			t.Fatalf("unexpected: %+v", req)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		t.Parallel()
		var req UpdatePaymentMethodRequest
		r := httptest.NewRequest(http.MethodPost, "/",
			strings.NewReader(`{"provider":"stripe","method_id":"pm_x","what":1}`))
		w := httptest.NewRecorder()
		if decodeJSON(w, r, &req) {
			t.Fatalf("want false on unknown field")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: want 400, got %d", w.Code)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()
		var req UpdatePaymentMethodRequest
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{not json}`))
		w := httptest.NewRecorder()
		if decodeJSON(w, r, &req) {
			t.Fatalf("want false on malformed")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: want 400, got %d", w.Code)
		}
	})
}

func TestParseUUIDParam(t *testing.T) {
	t.Parallel()
	t.Run("invalid uuid returns 400", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = setChiURLParam(r, "id", "not-a-uuid")
		w := httptest.NewRecorder()
		_, ok := parseUUIDParam(w, r, "id")
		if ok {
			t.Fatalf("want false")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: want 400, got %d", w.Code)
		}
	})

	t.Run("valid uuid", func(t *testing.T) {
		t.Parallel()
		want := uuid.New()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = setChiURLParam(r, "id", want.String())
		w := httptest.NewRecorder()
		got, ok := parseUUIDParam(w, r, "id")
		if !ok {
			t.Fatalf("want ok, got false (body=%s)", w.Body.String())
		}
		if got != want {
			t.Fatalf("uuid: want %s, got %s", want, got)
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantBody string
	}{
		{"not found", ErrSubscriptionNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"already exists", ErrAlreadyExists, http.StatusConflict, "ALREADY_EXISTS"},
		{"invalid transition", ErrInvalidTransition, http.StatusConflict, "INVALID_TRANSITION"},
		{"invalid provider", ErrInvalidProvider, http.StatusBadRequest, "INVALID_PROVIDER"},
		{"invalid pm", ErrInvalidPaymentMethod, http.StatusBadRequest, "INVALID_PAYMENT_METHOD"},
		{"invalid request", ErrInvalidRequest, http.StatusBadRequest, "BAD_REQUEST"},
		{"forbidden", ErrForbidden, http.StatusForbidden, "FORBIDDEN"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			writeError(w, c.err)
			if w.Code != c.wantCode {
				t.Fatalf("status: want %d, got %d", c.wantCode, w.Code)
			}
			var env struct {
				Error respond.ErrorBody `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode envelope: %v", err)
			}
			if env.Error.Code != c.wantBody {
				t.Fatalf("code: want %q, got %q", c.wantBody, env.Error.Code)
			}
		})
	}
}
