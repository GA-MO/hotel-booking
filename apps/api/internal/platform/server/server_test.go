package server

import (
	"log/slog"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/GA-MO/hotel-booking/apps/api/internal/config"
)

// TestRoutes_NoMountPanic verifies the route tree builds without chi panicking
// on a conflicting Mount(), and that every expected endpoint is registered.
// Handlers are constructed with nil pgx/redis pointers — that's safe because
// route REGISTRATION never touches the connection; only request execution does.
func TestRoutes_NoMountPanic(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:     "test-secret-32-bytes-long-or-more-please",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 720 * time.Hour,
		CORSOrigins:   []string{"http://localhost:3000"},
	}

	s := New(cfg, nil, nil, slog.Default())

	got := map[string]struct{}{}
	if err := chi.Walk(s.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	expected := []string{
		// health
		"GET /healthz",
		"GET /readyz",
		// auth
		"POST /v1/auth/signup",
		"POST /v1/auth/login",
		"POST /v1/auth/refresh",
		"POST /v1/auth/logout",
		"GET /v1/auth/me",
		// hotels top-level
		"GET /v1/hotels/",
		"POST /v1/hotels/",
		"GET /v1/hotels/slug-available",
		"GET /v1/hotels/{id}",
		"PATCH /v1/hotels/{id}",
		"DELETE /v1/hotels/{id}",
		// per-hotel — roomtype
		"GET /v1/hotels/{hotel_id}/room-types",
		"POST /v1/hotels/{hotel_id}/room-types",
		"GET /v1/hotels/{hotel_id}/room-types/{id}",
		"PATCH /v1/hotels/{hotel_id}/room-types/{id}",
		"DELETE /v1/hotels/{hotel_id}/room-types/{id}",
		"GET /v1/hotels/{hotel_id}/room-types/{id}/photos",
		"POST /v1/hotels/{hotel_id}/room-types/{id}/photos",
		"PATCH /v1/hotels/{hotel_id}/room-types/{id}/photos/{photo_id}",
		"DELETE /v1/hotels/{hotel_id}/room-types/{id}/photos/{photo_id}",
		"GET /v1/hotels/{hotel_id}/photos",
		"POST /v1/hotels/{hotel_id}/photos",
		"DELETE /v1/hotels/{hotel_id}/photos/{photo_id}",
		// per-hotel — pricing
		"GET /v1/hotels/{hotel_id}/availability",
		"PUT /v1/hotels/{hotel_id}/availability",
		"DELETE /v1/hotels/{hotel_id}/availability",
		"GET /v1/hotels/{hotel_id}/pricing-rules/",
		"POST /v1/hotels/{hotel_id}/pricing-rules/",
		"PATCH /v1/hotels/{hotel_id}/pricing-rules/{id}",
		"DELETE /v1/hotels/{hotel_id}/pricing-rules/{id}",
		// per-hotel — landing
		"GET /v1/hotels/{hotel_id}/landing/",
		"GET /v1/hotels/{hotel_id}/landing/{locale}",
		"PUT /v1/hotels/{hotel_id}/landing/{locale}",
		"POST /v1/hotels/{hotel_id}/landing/{locale}/publish",
		"POST /v1/hotels/{hotel_id}/landing/{locale}/unpublish",
		"DELETE /v1/hotels/{hotel_id}/landing/{locale}",
		// per-hotel — booking
		"GET /v1/hotels/{hotel_id}/bookings/",
		"POST /v1/hotels/{hotel_id}/bookings/",
		"GET /v1/hotels/{hotel_id}/bookings/{id}",
		"POST /v1/hotels/{hotel_id}/bookings/{id}/confirm",
		"POST /v1/hotels/{hotel_id}/bookings/{id}/cancel",
		"POST /v1/hotels/{hotel_id}/bookings/{id}/check-in",
		"POST /v1/hotels/{hotel_id}/bookings/{id}/check-out",
		"POST /v1/hotels/{hotel_id}/bookings/{id}/no-show",
		// public
		"POST /v1/public/quote/{slug}",
		"GET /v1/public/landing/{slug}/{locale}",
		"GET /v1/public/bookings/{reference}",
		"POST /v1/public/bookings/{reference}/cancel",
		"POST /v1/public/hotels/{slug}/bookings",
		// misc
		"GET /v1/ping",
	}

	for _, want := range expected {
		if _, ok := got[want]; !ok {
			t.Errorf("missing route: %s", want)
		}
	}

	if t.Failed() {
		gotList := make([]string, 0, len(got))
		for k := range got {
			gotList = append(gotList, k)
		}
		sort.Strings(gotList)
		t.Logf("Discovered routes (%d):", len(gotList))
		for _, r := range gotList {
			t.Logf("  %s", r)
		}
	}
}
