package booking

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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

// Routes returns the staff-facing bookings router. Mount under
// /v1/hotels/{hotel_id}/bookings — the handler reads hotel_id from URL params.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))

	r.Get("/", h.list)
	r.With(requireRole("owner", "manager", "front_desk")).Post("/", h.createWalkIn)
	r.Get("/{id}", h.get)
	r.With(requireRole("owner", "manager", "front_desk")).Get("/{id}/events", h.listEvents)
	r.With(requireRole("owner", "manager", "front_desk")).Post("/{id}/confirm", h.confirm)
	r.With(requireRole("owner", "manager", "front_desk")).Post("/{id}/cancel", h.cancel)
	r.With(requireRole("owner", "manager", "front_desk")).Post("/{id}/check-in", h.checkIn)
	r.With(requireRole("owner", "manager", "front_desk")).Post("/{id}/check-out", h.checkOut)
	r.With(requireRole("owner", "manager")).Post("/{id}/no-show", h.noShow)
	return r
}

// PublicRoutes returns the guest-facing booking router (no auth).
// Mount under /v1/public/bookings — the handler routes by reference + email.
// Plus /v1/public/hotels/{slug}/bookings for creation.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{reference}", h.publicGet)
	r.Post("/{reference}/cancel", h.publicCancel)
	r.Post("/{reference}/payment-confirmed", h.publicPaymentConfirmed)
	return r
}

// PublicCreateHandler is used by the parent server to attach `POST
// /v1/public/hotels/{slug}/bookings` — it needs the slug to resolve the
// hotel and that mount lives in server.go.
func (h *Handler) PublicCreateHandler(resolveHotelBySlug func(string) (uuid.UUID, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		hotelID, ok := resolveHotelBySlug(slug)
		if !ok {
			respond.Error(w, http.StatusNotFound, "NOT_FOUND", "hotel not found")
			return
		}
		var req CreateRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		b, err := h.svc.CreatePublic(r.Context(), hotelID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		respond.Body(w, http.StatusCreated, b)
	}
}

// ----- staff handlers -----

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseHotelID(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	bookings, err := h.svc.ListByHotel(r.Context(), identity.AccountID, hotelID, status, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, ListResponse{Bookings: bookings, Total: len(bookings)})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	b, err := h.svc.GetForAccount(r.Context(), identity.AccountID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) createWalkIn(w http.ResponseWriter, r *http.Request) {
	hotelID, ok := parseHotelID(w, r)
	if !ok {
		return
	}
	var req CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := h.svc.CreateInternal(r.Context(), hotelID, req, SourceWalkIn)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, b)
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	b, err := h.svc.Confirm(r.Context(), id, "hotel_staff", &identity.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req CancelRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body is optional
	b, err := h.svc.CancelByHotel(r.Context(), id, req.Reason, identity.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) checkIn(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	b, err := h.svc.CheckIn(r.Context(), id, identity.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) checkOut(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	b, err := h.svc.CheckOut(r.Context(), id, identity.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) noShow(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	b, err := h.svc.NoShow(r.Context(), id, identity.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseHotelID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	events, err := h.svc.ListEvents(r.Context(), identity.AccountID, hotelID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, BookingEventsResponse{Events: events})
}

// ----- public handlers -----

func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	reference := chi.URLParam(r, "reference")
	email := r.URL.Query().Get("email")
	if email == "" {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "email query parameter required")
		return
	}
	b, err := h.svc.GetPublic(r.Context(), reference, email)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) publicCancel(w http.ResponseWriter, r *http.Request) {
	reference := chi.URLParam(r, "reference")
	var req struct {
		Email  string `json:"email"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := h.svc.CancelByGuest(r.Context(), reference, req.Email, req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
}

func (h *Handler) publicPaymentConfirmed(w http.ResponseWriter, r *http.Request) {
	reference := chi.URLParam(r, "reference")
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := h.svc.MarkPaymentClaimed(r.Context(), reference, req.Email)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, b)
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

func parseHotelID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return parseUUIDParam(w, r, "hotel_id")
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
	case errors.Is(err, ErrBookingNotFound), errors.Is(err, ErrRoomTypeNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "not found")
	case errors.Is(err, ErrInvalidDates):
		respond.Error(w, http.StatusBadRequest, "INVALID_DATES", "check_in_date must be before check_out_date (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidGuest):
		respond.Error(w, http.StatusBadRequest, "INVALID_GUEST", "guest email and name are required")
	case errors.Is(err, ErrInvalidRoomCount):
		respond.Error(w, http.StatusBadRequest, "INVALID_ROOM_COUNT", "room_count must be >= 1")
	case errors.Is(err, ErrNoAvailability):
		respond.Error(w, http.StatusConflict, "NO_AVAILABILITY", "no rooms available for the requested dates")
	case errors.Is(err, ErrInvalidStateTransition):
		respond.Error(w, http.StatusConflict, "INVALID_STATE", "booking is not in the right state for this action")
	case errors.Is(err, ErrHotelNotLive):
		respond.Error(w, http.StatusConflict, "HOTEL_NOT_LIVE", "hotel is not accepting bookings")
	default:
		slog.Error("booking handler", "err", err.Error())
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
