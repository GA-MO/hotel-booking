package roomtype

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/httpx"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

// Thin aliases so existing call sites keep their compact form; the actual
// helpers live in platform/httpx so behavior changes (e.g. an audit log on
// role check) land everywhere at once.
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

// Routes mounts the room-types + photos sub-router. The parent route is expected
// to be `/v1/hotels/{hotel_id}`. All endpoints require an authenticated identity.
func (h *Handler) Routes(jwt *auth.JWT) chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(jwt))
	h.attach(r)
	return r
}

// AttachTo registers the room-type + photo routes on an existing chi.Router.
// Use this when the parent already applies auth middleware and wants to mix
// these routes with other modules under the same prefix (e.g. /hotels/{hotel_id}).
// Caller is responsible for ensuring auth.RequireAuth is in place upstream.
func (h *Handler) AttachTo(r chi.Router) {
	h.attach(r)
}

func (h *Handler) attach(r chi.Router) {
	// room types
	r.Get("/room-types", h.listRoomTypes)
	r.With(requireRole("owner", "manager")).Post("/room-types", h.createRoomType)
	r.Get("/room-types/{id}", h.getRoomType)
	r.With(requireRole("owner", "manager")).Patch("/room-types/{id}", h.updateRoomType)
	r.With(requireRole("owner")).Delete("/room-types/{id}", h.deleteRoomType)

	// room-type photos
	r.Get("/room-types/{id}/photos", h.listRoomTypePhotos)
	r.With(requireRole("owner", "manager")).Post("/room-types/{id}/photos", h.createRoomTypePhoto)
	r.With(requireRole("owner", "manager")).Patch("/room-types/{id}/photos/{photo_id}", h.updateRoomTypePhoto)
	r.With(requireRole("owner", "manager")).Delete("/room-types/{id}/photos/{photo_id}", h.deleteRoomTypePhoto)

	// hotel-level photos
	r.Get("/photos", h.listHotelPhotos)
	r.With(requireRole("owner", "manager")).Post("/photos", h.createHotelPhoto)
	r.With(requireRole("owner", "manager")).Delete("/photos/{photo_id}", h.deleteHotelPhoto)
}

// ----- room types -----

func (h *Handler) listRoomTypes(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	rts, err := h.svc.List(r.Context(), identity.AccountID, hotelID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, ListResponse{RoomTypes: rts})
}

func (h *Handler) createRoomType(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	var req CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rt, err := h.svc.Create(r.Context(), identity.AccountID, hotelID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, rt)
}

func (h *Handler) getRoomType(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	rt, err := h.svc.Get(r.Context(), identity.AccountID, hotelID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, rt)
}

func (h *Handler) updateRoomType(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req UpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rt, err := h.svc.Update(r.Context(), identity.AccountID, hotelID, id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, rt)
}

func (h *Handler) deleteRoomType(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), identity.AccountID, hotelID, id); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

// ----- room-type photos -----

func (h *Handler) listRoomTypePhotos(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	roomTypeID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	photos, err := h.svc.ListRoomTypePhotos(r.Context(), identity.AccountID, hotelID, roomTypeID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, PhotoListResponse{Photos: photos})
}

func (h *Handler) createRoomTypePhoto(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	roomTypeID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req CreatePhotoRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := h.svc.CreateRoomTypePhoto(r.Context(), identity.AccountID, hotelID, roomTypeID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, p)
}

func (h *Handler) updateRoomTypePhoto(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	roomTypeID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	photoID, ok := parseUUIDParam(w, r, "photo_id")
	if !ok {
		return
	}
	var req UpdatePhotoRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := h.svc.UpdateRoomTypePhoto(r.Context(), identity.AccountID, hotelID, roomTypeID, photoID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, p)
}

func (h *Handler) deleteRoomTypePhoto(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	roomTypeID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	photoID, ok := parseUUIDParam(w, r, "photo_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteRoomTypePhoto(r.Context(), identity.AccountID, hotelID, roomTypeID, photoID); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}

// ----- hotel photos -----

func (h *Handler) listHotelPhotos(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	photos, err := h.svc.ListHotelPhotos(r.Context(), identity.AccountID, hotelID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, PhotoListResponse{Photos: photos})
}

func (h *Handler) createHotelPhoto(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	var req CreatePhotoRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := h.svc.CreateHotelPhoto(r.Context(), identity.AccountID, hotelID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusCreated, p)
}

func (h *Handler) deleteHotelPhoto(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	hotelID, ok := parseUUIDParam(w, r, "hotel_id")
	if !ok {
		return
	}
	photoID, ok := parseUUIDParam(w, r, "photo_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteHotelPhoto(r.Context(), identity.AccountID, hotelID, photoID); err != nil {
		writeError(w, err)
		return
	}
	respond.Empty(w, http.StatusNoContent)
}


func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRoomTypeNotFound):
		// Cross-tenant access deliberately maps here (not 403) to avoid
		// disclosing whether a row exists for another account.
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "room type not found")
	case errors.Is(err, ErrPhotoNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "photo not found")
	case errors.Is(err, ErrInvalidName):
		respond.Error(w, http.StatusBadRequest, "INVALID_NAME", "name is required and must be 1-120 chars")
	case errors.Is(err, ErrInvalidCapacity):
		respond.Error(w, http.StatusBadRequest, "INVALID_CAPACITY", "max_occupancy must be >= 1")
	case errors.Is(err, ErrInvalidInventory):
		respond.Error(w, http.StatusBadRequest, "INVALID_INVENTORY", "total_inventory must be >= 0")
	case errors.Is(err, ErrInvalidBaseRate):
		respond.Error(w, http.StatusBadRequest, "INVALID_BASE_RATE", "base_rate must be >= 0")
	case errors.Is(err, ErrInvalidCurrency):
		respond.Error(w, http.StatusBadRequest, "INVALID_CURRENCY", "base_currency must be a 3-letter ISO 4217 code")
	case errors.Is(err, ErrInvalidDisplayOrder):
		respond.Error(w, http.StatusBadRequest, "INVALID_DISPLAY_ORDER", "display_order must be >= 0")
	case errors.Is(err, ErrInvalidStorageKey):
		respond.Error(w, http.StatusBadRequest, "INVALID_STORAGE_KEY", "storage_key is required")
	case errors.Is(err, ErrCoverConflict):
		respond.Error(w, http.StatusConflict, "COVER_CONFLICT", "another photo is already marked as cover")
	case errors.Is(err, ErrForbidden):
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
