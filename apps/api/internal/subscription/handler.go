package subscription

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

// Routes mounts /v1/subscription. All endpoints are account-scoped — no
// hotel_id in the path. Auth is required throughout.
//
// Admin-only operations (suspend, terminate, reactivate) are intentionally
// not exposed: they happen automatically via the worker scan. A future
// platform-admin module can wrap Service directly.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))

	r.Get("/", h.get)
	r.With(requireRole("owner")).Post("/payment-method", h.updatePaymentMethod)
	r.With(requireRole("owner")).Post("/cancel", h.cancel)

	return r
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	sub, err := h.svc.Get(r.Context(), identity.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, sub)
}

func (h *Handler) updatePaymentMethod(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	var req UpdatePaymentMethodRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	actorID := identity.UserID
	sub, err := h.svc.UpdatePaymentMethod(r.Context(), identity.AccountID, &actorID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, sub)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFrom(r.Context())
	var req CancelRequest
	// Body is optional — empty body means no reason. Don't fail on EOF.
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	actorID := identity.UserID
	sub, err := h.svc.Cancel(r.Context(), identity.AccountID, &actorID, req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, sub)
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

// parseUUIDParam is kept for parity with other modules even though the
// subscription handler has no path params today. Future admin routes
// (e.g. GET /v1/admin/subscriptions/{id}) will use it.
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
	case errors.Is(err, ErrSubscriptionNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "subscription not found")
	case errors.Is(err, ErrAlreadyExists):
		respond.Error(w, http.StatusConflict, "ALREADY_EXISTS", "subscription already exists for account")
	case errors.Is(err, ErrInvalidTransition):
		respond.Error(w, http.StatusConflict, "INVALID_TRANSITION", "subscription state transition not allowed")
	case errors.Is(err, ErrInvalidProvider):
		respond.Error(w, http.StatusBadRequest, "INVALID_PROVIDER", "provider must be 'stripe' or 'omise'")
	case errors.Is(err, ErrInvalidPaymentMethod):
		respond.Error(w, http.StatusBadRequest, "INVALID_PAYMENT_METHOD", "invalid payment method fields")
	case errors.Is(err, ErrInvalidRequest):
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request")
	case errors.Is(err, ErrForbidden):
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
