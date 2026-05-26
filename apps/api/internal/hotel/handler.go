package hotel

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

// Routes mounts /v1/hotels. All endpoints require an authenticated identity.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))

	r.Get("/", h.list)
	r.With(requireRole("owner", "manager")).Post("/", h.create)
	r.Get("/slug-available", h.slugAvailable)
	r.Get("/{id}", h.get)
	r.With(requireRole("owner", "manager")).Patch("/{id}", h.update)
	r.With(requireRole("owner")).Delete("/{id}", h.delete)

	return r
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	var req CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	hotel, err := h.svc.Create(r.Context(), identity.AccountID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, hotel)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotels, err := h.svc.List(r.Context(), identity.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, ListResponse{Hotels: hotels})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	hotel, err := h.svc.Get(r.Context(), identity.AccountID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, hotel)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	hotel, err := h.svc.Update(r.Context(), identity.AccountID, id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, hotel)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), identity.AccountID, id); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

func (h *Handler) slugAvailable(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "slug query parameter required")
		return
	}
	available, err := h.svc.SlugAvailable(r.Context(), slug)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, SlugAvailableResponse{Slug: slug, Available: available})
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
	case errors.Is(err, ErrHotelNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "hotel not found")
	case errors.Is(err, ErrSlugAlreadyTaken):
		respond.Error(w, http.StatusConflict, "SLUG_TAKEN", "this slug is already in use")
	case errors.Is(err, ErrInvalidSlug):
		respond.Error(w, http.StatusBadRequest, "INVALID_SLUG", "slug must be 3-80 chars, lowercase letters/digits/hyphens, no leading/trailing hyphen")
	case errors.Is(err, ErrInvalidName):
		respond.Error(w, http.StatusBadRequest, "INVALID_NAME", "name is required and must be ≤255 chars")
	case errors.Is(err, ErrInvalidPromptPayID):
		respond.Error(w, http.StatusBadRequest, "INVALID_PROMPTPAY_ID", "promptpay_id must be a 10-digit phone, 13-digit national ID, or 15-char tax ID")
	case errors.Is(err, ErrForbidden):
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
