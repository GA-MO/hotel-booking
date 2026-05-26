package landing

import (
	"encoding/json"
	"errors"
	"net/http"

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

// Routes mounts the admin endpoints under /v1/hotels/{hotel_id}/landing.
// All endpoints require an authenticated identity; tenant isolation is
// enforced inside the service (mismatched hotel_id → 404).
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))

	r.Get("/", h.list)
	r.Get("/{locale}", h.get)
	r.With(requireRole("owner", "manager")).Put("/{locale}", h.upsert)
	r.With(requireRole("owner", "manager")).Post("/{locale}/publish", h.publish)
	r.With(requireRole("owner")).Post("/{locale}/unpublish", h.unpublish)
	r.With(requireRole("owner")).Delete("/{locale}", h.delete)

	return r
}

// PublicRoutes mounts the unauthenticated read endpoint used by the booking-web
// ISR build. Mounted by the server under /v1/public/landing.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{slug}/{locale}", h.publicGet)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	pages, err := h.svc.List(r.Context(), identity.AccountID, hotelID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, ListResponse{LandingPages: pages})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	locale := chi.URLParam(r, "locale")
	page, err := h.svc.Get(r.Context(), identity.AccountID, hotelID, locale)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, page)
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	locale := chi.URLParam(r, "locale")
	var req UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	page, err := h.svc.Upsert(r.Context(), identity.AccountID, hotelID, locale, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, page)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	locale := chi.URLParam(r, "locale")
	page, err := h.svc.Publish(r.Context(), identity.AccountID, hotelID, locale)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, page)
}

func (h *Handler) unpublish(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	locale := chi.URLParam(r, "locale")
	page, err := h.svc.Unpublish(r.Context(), identity.AccountID, hotelID, locale)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, page)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	locale := chi.URLParam(r, "locale")
	if err := h.svc.Delete(r.Context(), identity.AccountID, hotelID, locale); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

// publicGet serves /v1/public/landing/{slug}/{locale}. No auth, only returns
// published rows on live hotels — see Repository.GetPublishedBySlug for the
// safety net.
func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	locale := chi.URLParam(r, "locale")
	page, err := h.svc.GetPublishedBySlug(r.Context(), slug, locale)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, page)
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
	case errors.Is(err, ErrLandingPageNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "landing page not found")
	case errors.Is(err, ErrHotelNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "hotel not found")
	case errors.Is(err, ErrInvalidLocale):
		respond.Error(w, http.StatusBadRequest, "INVALID_LOCALE", "locale must match ^[a-z]{2}(-[A-Z]{2})?$")
	case errors.Is(err, ErrInvalidColor):
		respond.Error(w, http.StatusBadRequest, "INVALID_COLOR", "color must be #RRGGBB hex")
	case errors.Is(err, ErrInvalidSectionType):
		respond.Error(w, http.StatusBadRequest, "INVALID_SECTION_TYPE", "unknown section type")
	case errors.Is(err, ErrInvalidSection):
		respond.Error(w, http.StatusBadRequest, "INVALID_SECTION", "invalid section payload")
	case errors.Is(err, ErrInvalidRequest):
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request")
	case errors.Is(err, ErrForbidden):
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
