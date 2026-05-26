package pricing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
					t.Fatalf("decode envelope: %v", err)
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
		var req CreateRuleRequest
		body := `{"name":"W","rule_type":"day_of_week","modifier_type":"percentage","modifier_value":"20","days_of_week":[5,6]}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		if !decodeJSON(w, r, &req) {
			t.Fatalf("want ok (body=%s)", w.Body.String())
		}
		if req.Name != "W" || req.RuleType != RuleTypeDayOfWeek {
			t.Fatalf("decoded: %+v", req)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		t.Parallel()
		var req CreateRuleRequest
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"W","what":1}`))
		w := httptest.NewRecorder()
		if decodeJSON(w, r, &req) {
			t.Fatalf("want false")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: want 400, got %d", w.Code)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()
		var req CreateRuleRequest
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{not json}`))
		w := httptest.NewRecorder()
		if decodeJSON(w, r, &req) {
			t.Fatalf("want false")
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
			t.Fatalf("want ok (body=%s)", w.Body.String())
		}
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})
}

func TestParseDateQuery(t *testing.T) {
	t.Parallel()
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		if _, ok := parseDateQuery(w, r, "start"); ok {
			t.Fatalf("want false")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: want 400, got %d", w.Code)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		u, _ := url.Parse("/?start=2025/06/01")
		r := &http.Request{Method: http.MethodGet, URL: u, Header: http.Header{}}
		w := httptest.NewRecorder()
		if _, ok := parseDateQuery(w, r, "start"); ok {
			t.Fatalf("want false")
		}
	})
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		u, _ := url.Parse("/?start=2025-06-01")
		r := &http.Request{Method: http.MethodGet, URL: u, Header: http.Header{}}
		w := httptest.NewRecorder()
		got, ok := parseDateQuery(w, r, "start")
		if !ok {
			t.Fatalf("want ok (body=%s)", w.Body.String())
		}
		if got.Format(DateLayout) != "2025-06-01" {
			t.Fatalf("got %s", got.Format(DateLayout))
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err      error
		wantCode int
	}{
		{ErrHotelNotFound, http.StatusNotFound},
		{ErrRoomTypeNotFound, http.StatusNotFound},
		{ErrRuleNotFound, http.StatusNotFound},
		{ErrInvalidDate, http.StatusBadRequest},
		{ErrInvalidDateRange, http.StatusBadRequest},
		{ErrInvalidRooms, http.StatusBadRequest},
		{ErrInvalidRule, http.StatusBadRequest},
		{ErrInvalidOverride, http.StatusBadRequest},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		writeError(w, c.err)
		if w.Code != c.wantCode {
			t.Errorf("%v: want %d, got %d", c.err, c.wantCode, w.Code)
		}
	}
}
