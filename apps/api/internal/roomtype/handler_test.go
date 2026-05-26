package roomtype

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
		{"role not allowed", "front_desk", true, []string{"owner", "manager"}, http.StatusForbidden, "FORBIDDEN"},
		{"role allowed (owner)", "owner", true, []string{"owner", "manager"}, http.StatusOK, ""},
		{"role allowed (manager)", "manager", true, []string{"owner", "manager"}, http.StatusOK, ""},
		{"read_only blocked from owner-only", "read_only", true, []string{"owner"}, http.StatusForbidden, "FORBIDDEN"},
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
		var req CreateRequest
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
			`{"name":"Deluxe","total_inventory":4,"max_occupancy":2,"base_rate":1200}`,
		))
		w := httptest.NewRecorder()
		if !decodeJSON(w, r, &req) {
			t.Fatalf("want ok, got false (body=%s)", w.Body.String())
		}
		if req.Name != "Deluxe" || req.TotalInventory != 4 || req.MaxOccupancy != 2 || req.BaseRate != 1200 {
			t.Fatalf("unexpected: %+v", req)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		t.Parallel()
		var req CreateRequest
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x","what":1}`))
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
		var req CreateRequest
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

	t.Run("missing param", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = setChiURLParam(r, "other", uuid.New().String())
		w := httptest.NewRecorder()
		_, ok := parseUUIDParam(w, r, "id")
		if ok {
			t.Fatalf("want false on missing param")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: want 400, got %d", w.Code)
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", ErrRoomTypeNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"photo not found", ErrPhotoNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"invalid name", ErrInvalidName, http.StatusBadRequest, "INVALID_NAME"},
		{"invalid capacity", ErrInvalidCapacity, http.StatusBadRequest, "INVALID_CAPACITY"},
		{"invalid inventory", ErrInvalidInventory, http.StatusBadRequest, "INVALID_INVENTORY"},
		{"invalid base rate", ErrInvalidBaseRate, http.StatusBadRequest, "INVALID_BASE_RATE"},
		{"invalid currency", ErrInvalidCurrency, http.StatusBadRequest, "INVALID_CURRENCY"},
		{"invalid display order", ErrInvalidDisplayOrder, http.StatusBadRequest, "INVALID_DISPLAY_ORDER"},
		{"invalid storage key", ErrInvalidStorageKey, http.StatusBadRequest, "INVALID_STORAGE_KEY"},
		{"cover conflict", ErrCoverConflict, http.StatusConflict, "COVER_CONFLICT"},
		{"forbidden", ErrForbidden, http.StatusForbidden, "FORBIDDEN"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			writeError(w, c.err)
			if w.Code != c.wantStatus {
				t.Fatalf("status: want %d, got %d", c.wantStatus, w.Code)
			}
			var env struct {
				Error respond.ErrorBody `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode envelope: %v", err)
			}
			if env.Error.Code != c.wantCode {
				t.Fatalf("code: want %q, got %q", c.wantCode, env.Error.Code)
			}
		})
	}
}
