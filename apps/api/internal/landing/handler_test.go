package landing

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
		{"role allowed", "owner", true, []string{"owner", "manager"}, http.StatusOK, ""},
		{"second role allowed", "manager", true, []string{"owner", "manager"}, http.StatusOK, ""},
		{"owner-only blocks manager", "manager", true, []string{"owner"}, http.StatusForbidden, "FORBIDDEN"},
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

	t.Run("valid empty update", func(t *testing.T) {
		t.Parallel()
		var req UpdateRequest
		body := `{"branding":{},"sections":[],"seo":{},"tracking":{}}`
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		if !decodeJSON(w, r, &req) {
			t.Fatalf("want ok, got false (body=%s)", w.Body.String())
		}
		if req.Sections == nil {
			t.Fatalf("sections should decode as empty slice, got nil")
		}
		if len(req.Sections) != 0 {
			t.Fatalf("sections: want empty, got %v", req.Sections)
		}
	})

	t.Run("valid populated update", func(t *testing.T) {
		t.Parallel()
		var req UpdateRequest
		body := `{
			"branding":{"primary_color":"#ff00aa","logo_url":"https://x/y.png"},
			"sections":[{"type":"hero","enabled":true,"order":0,"content":{"headline":"Welcome"}}],
			"seo":{"title":"Z"},
			"tracking":{"facebook_pixel_id":"123"}
		}`
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		if !decodeJSON(w, r, &req) {
			t.Fatalf("want ok, got false (body=%s)", w.Body.String())
		}
		if req.Branding.PrimaryColor != "#ff00aa" {
			t.Fatalf("primary_color: want #ff00aa, got %q", req.Branding.PrimaryColor)
		}
		if len(req.Sections) != 1 || req.Sections[0].Type != "hero" {
			t.Fatalf("sections decoded wrong: %+v", req.Sections)
		}
		if req.Tracking.FacebookPixelID != "123" {
			t.Fatalf("fb pixel: want 123, got %q", req.Tracking.FacebookPixelID)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		t.Parallel()
		var req UpdateRequest
		body := `{"branding":{},"sections":[],"seo":{},"tracking":{},"extra":1}`
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
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
		var req UpdateRequest
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{not json}`))
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
		r = setChiURLParam(r, "hotel_id", "not-a-uuid")
		w := httptest.NewRecorder()
		_, ok := parseUUIDParam(w, r, "hotel_id")
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
		r = setChiURLParam(r, "hotel_id", want.String())
		w := httptest.NewRecorder()
		got, ok := parseUUIDParam(w, r, "hotel_id")
		if !ok {
			t.Fatalf("want ok, got false (body=%s)", w.Body.String())
		}
		if got != want {
			t.Fatalf("uuid: want %s, got %s", want, got)
		}
	})
}

func TestWriteErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"landing not found", ErrLandingPageNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"hotel not found", ErrHotelNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"invalid locale", ErrInvalidLocale, http.StatusBadRequest, "INVALID_LOCALE"},
		{"invalid color", ErrInvalidColor, http.StatusBadRequest, "INVALID_COLOR"},
		{"invalid section type", ErrInvalidSectionType, http.StatusBadRequest, "INVALID_SECTION_TYPE"},
		{"invalid section", ErrInvalidSection, http.StatusBadRequest, "INVALID_SECTION"},
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
				t.Fatalf("decode error envelope: %v", err)
			}
			if env.Error.Code != c.wantCode {
				t.Fatalf("code: want %q, got %q", c.wantCode, env.Error.Code)
			}
		})
	}
}
