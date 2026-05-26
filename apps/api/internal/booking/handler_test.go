package booking

import (
	"encoding/json"
	"errors"
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
		role         string
		hasIdentity  bool
		allowed      []string
		wantStatus   int
		wantCode     string
	}{
		{"no identity", "", false, []string{"owner"}, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"role denied", "read_only", true, []string{"owner", "manager"}, http.StatusForbidden, "FORBIDDEN"},
		{"role allowed (front_desk)", "front_desk", true, []string{"owner", "manager", "front_desk"}, http.StatusOK, ""},
		{"role allowed (owner)", "owner", true, []string{"owner"}, http.StatusOK, ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := requireRole(c.allowed...)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.hasIdentity {
				req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{
					UserID: uuid.New(), AccountID: uuid.New(), Role: c.role,
				}))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != c.wantStatus {
				t.Fatalf("status: want %d, got %d", c.wantStatus, rec.Code)
			}
			if c.wantCode != "" {
				var env struct{ Error respond.ErrorBody `json:"error"` }
				if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if env.Error.Code != c.wantCode {
					t.Fatalf("code: want %q, got %q", c.wantCode, env.Error.Code)
				}
			}
		})
	}
}

func TestDecodeJSON_RejectsUnknownField(t *testing.T) {
	t.Parallel()
	var req CreateRequest
	r := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"room_count":1,"unknown":"x"}`))
	w := httptest.NewRecorder()
	if decodeJSON(w, r, &req) {
		t.Fatalf("want false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestWriteError_Mapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{ErrBookingNotFound, http.StatusNotFound, "NOT_FOUND"},
		{ErrRoomTypeNotFound, http.StatusNotFound, "NOT_FOUND"},
		{ErrInvalidDates, http.StatusBadRequest, "INVALID_DATES"},
		{ErrInvalidGuest, http.StatusBadRequest, "INVALID_GUEST"},
		{ErrInvalidRoomCount, http.StatusBadRequest, "INVALID_ROOM_COUNT"},
		{ErrNoAvailability, http.StatusConflict, "NO_AVAILABILITY"},
		{ErrInvalidStateTransition, http.StatusConflict, "INVALID_STATE"},
		{ErrHotelNotLive, http.StatusConflict, "HOTEL_NOT_LIVE"},
		{errors.New("anything else"), http.StatusInternalServerError, "INTERNAL"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.wantCode, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			writeError(w, c.err)
			if w.Code != c.wantStatus {
				t.Fatalf("status: want %d, got %d", c.wantStatus, w.Code)
			}
			var env struct{ Error respond.ErrorBody `json:"error"` }
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if env.Error.Code != c.wantCode {
				t.Fatalf("code: want %q, got %q", c.wantCode, env.Error.Code)
			}
		})
	}
}
