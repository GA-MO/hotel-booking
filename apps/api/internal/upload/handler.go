package upload

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

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

// Routes mounts /v1/uploads. Both endpoints require an authenticated identity
// (any role — read_only included — uploads happen during the onboarding
// wizard too).
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))

	r.Post("/presign", h.presign)
	r.Post("/imgproxy-url", h.imgproxyURL)

	return r
}

func (h *Handler) presign(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	var req PresignRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.Presign(r.Context(), identity.AccountID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, resp)
}

func (h *Handler) imgproxyURL(w http.ResponseWriter, r *http.Request) {
	var req ImgproxyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.svc.ImgproxyURL(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, resp)
}

// ----- helpers -----

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
	status, code, msg := mapErrorCode(err)
	respond.Error(w, status, code, msg)
}
