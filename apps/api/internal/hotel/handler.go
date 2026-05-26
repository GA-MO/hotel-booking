package hotel

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/httpx"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

// Thin aliases so the existing handler call sites + test files keep their
// compact form. Real implementations live in platform/httpx.
var (
	requireRole    = httpx.RequireRole
	parseUUIDParam = httpx.ParseUUIDParam
	decodeJSON     = httpx.DecodeJSON
)

type Handler struct {
	svc *Service
	jwt *auth.JWT
}

func NewHandler(svc *Service, jwt *auth.JWT) *Handler {
	return &Handler{svc: svc, jwt: jwt}
}

// AttachCollection registers the collection endpoints (list / create /
// slug-available) onto a router rooted at /hotels.
func (h *Handler) AttachCollection(r chi.Router) {
	r.Get("/", h.list)
	r.With(requireRole("owner", "manager")).Post("/", h.create)
	r.Get("/slug-available", h.slugAvailable)
}

// AttachByID registers the by-id endpoints (get / update / delete) onto a
// router rooted at /hotels/{hotel_id}. The parent owns the `{hotel_id}` path
// param — splitting collection vs by-id this way avoids chi's shadow when a
// sibling Mount + Route("/{x}") sit at the same parent.
func (h *Handler) AttachByID(r chi.Router) {
	r.Get("/", h.get)
	r.With(requireRole("owner", "manager")).Patch("/", h.update)
	r.With(requireRole("owner")).Delete("/", h.delete)
	r.With(requireRole("owner")).Post("/go-live", h.goLive)
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
	id, ok := parseUUIDParam(w, r, "hotel_id")
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
	id, ok := parseUUIDParam(w, r, "hotel_id")
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
	id, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), identity.AccountID, id); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

func (h *Handler) goLive(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	id, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	hotel, err := h.svc.GoLive(r.Context(), identity.AccountID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, hotel)
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
	case errors.Is(err, ErrInvalidStateTransition):
		respond.Error(w, http.StatusConflict, "INVALID_STATE_TRANSITION", "hotel status cannot transition from its current state")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
