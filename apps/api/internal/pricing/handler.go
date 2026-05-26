package pricing

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

type Handler struct {
	svc *Service
	jwt *auth.JWT
}

func NewHandler(svc *Service, jwt *auth.JWT) *Handler {
	return &Handler{svc: svc, jwt: jwt}
}

// Routes mounts admin endpoints under /v1/hotels/{hotel_id}. All endpoints
// require authentication; mutating endpoints additionally require an owner or
// manager role. The parent task wires this router at the right path; we
// expect chi.URLParam(r, "hotel_id") to resolve.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))
	h.attach(r)
	return r
}

// AttachTo registers availability + pricing-rules routes on an existing
// chi.Router. Use this when the parent has already applied
// auth.RequireAuth and wants to share /hotels/{hotel_id} with other modules.
func (h *Handler) AttachTo(r chi.Router) {
	h.attach(r)
}

func (h *Handler) attach(r chi.Router) {
	// Availability
	r.Get("/availability", h.getAvailability)
	r.With(requireRole("owner", "manager")).Put("/availability", h.upsertAvailability)
	r.With(requireRole("owner", "manager")).Delete("/availability", h.deleteAvailability)

	// Pricing rules
	r.Route("/pricing-rules", func(rr chi.Router) {
		rr.Get("/", h.listRules)
		rr.With(requireRole("owner", "manager")).Post("/", h.createRule)
		rr.With(requireRole("owner", "manager")).Patch("/{id}", h.updateRule)
		rr.With(requireRole("owner", "manager")).Delete("/{id}", h.deleteRule)
	})
}

// PublicRoutes mounts unauthenticated guest-side endpoints. Mount the returned
// router at /v1/public/quote — the parent supplies the /quote segment so chi's
// route Walk doesn't print a `*` wildcard for a root-mounted sub-tree.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{slug}", h.publicQuote)
	return r
}

// ----- availability handlers -----

func (h *Handler) getAvailability(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	start, ok := parseDateQuery(w, r, "start")
	if !ok {
		return
	}
	end, ok := parseDateQuery(w, r, "end")
	if !ok {
		return
	}
	resp, err := h.svc.GetAvailability(r.Context(), identity.AccountID, hotelID, start, end)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, resp)
}

func (h *Handler) upsertAvailability(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	var req UpsertAvailabilityRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.UpsertAvailability(r.Context(), identity.AccountID, hotelID, req); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

func (h *Handler) deleteAvailability(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	rtRaw := r.URL.Query().Get("room_type_id")
	roomTypeID, err := uuid.Parse(rtRaw)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid room_type_id")
		return
	}
	dateRaw := r.URL.Query().Get("date")
	date, err := time.Parse(DateLayout, dateRaw)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid date")
		return
	}
	if err := h.svc.DeleteAvailabilityOverride(r.Context(), identity.AccountID, hotelID, roomTypeID, date); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

// ----- pricing rules handlers -----

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	rules, err := h.svc.ListRules(r.Context(), identity.AccountID, hotelID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, RuleListResponse{Rules: rules})
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	var req CreateRuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, err := h.svc.CreateRule(r.Context(), identity.AccountID, hotelID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, rule)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpdateRuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, err := h.svc.UpdateRule(r.Context(), identity.AccountID, hotelID, id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, rule)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteRule(r.Context(), identity.AccountID, hotelID, id); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

// ----- public handlers -----

func (h *Handler) publicQuote(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "missing slug")
		return
	}
	var req QuoteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.PublicQuote(r.Context(), slug, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, resp)
}

// ----- helpers -----

func requireRole(allowed ...string) func(http.Handler) http.Handler {
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

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

func parseDateQuery(w http.ResponseWriter, r *http.Request, key string) (time.Time, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "missing "+key+" (YYYY-MM-DD)")
		return time.Time{}, false
	}
	t, err := time.Parse(DateLayout, raw)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid "+key+" (YYYY-MM-DD)")
		return time.Time{}, false
	}
	return t, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrHotelNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "hotel not found")
	case errors.Is(err, ErrRoomTypeNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "room type not found")
	case errors.Is(err, ErrRuleNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "pricing rule not found")
	case errors.Is(err, ErrInvalidDate):
		respond.Error(w, http.StatusBadRequest, "INVALID_DATE", "invalid date (expected YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDateRange):
		respond.Error(w, http.StatusBadRequest, "INVALID_DATE_RANGE", "check_out must be after check_in")
	case errors.Is(err, ErrInvalidRooms):
		respond.Error(w, http.StatusBadRequest, "INVALID_ROOMS", "rooms must be ≥ 1 and ≤ 50")
	case errors.Is(err, ErrInvalidRule):
		respond.Error(w, http.StatusBadRequest, "INVALID_RULE", trimErr(err, "pricing rule"))
	case errors.Is(err, ErrInvalidOverride):
		respond.Error(w, http.StatusBadRequest, "INVALID_OVERRIDE", trimErr(err, "availability override"))
	case errors.Is(err, ErrInvalidRequest):
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", trimErr(err, "invalid request"))
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}

// trimErr returns the wrapped error's message (post the sentinel prefix) when
// useful for the client; otherwise falls back to fallback.
func trimErr(err error, fallback string) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	if msg == "" {
		return fallback
	}
	return msg
}
