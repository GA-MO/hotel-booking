package notification

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/httpx"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

var (
	requireRole    = httpx.RequireRole
	parseUUIDParam = httpx.ParseUUIDParam
	decodeJSON     = httpx.DecodeJSON
)

type Handler struct {
	svc    *Service
	jwt    *auth.JWT
	sender Sender // used for the debug /test endpoint to send synchronously
}

func NewHandler(svc *Service, jwt *auth.JWT, sender Sender) *Handler {
	return &Handler{svc: svc, jwt: jwt, sender: sender}
}

// Routes returns the admin-facing notifications router. The parent server is
// expected to mount this under /v1/notifications.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAuth(h.jwt))

	r.With(requireRole("owner", "manager")).Get("/", h.list)
	r.With(requireRole("owner", "manager")).Get("/{id}", h.get)
	r.With(requireRole("owner")).Post("/test", h.test)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	out, err := h.svc.List(r.Context(), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, ListResponse{Notifications: out, Total: len(out)})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.Body(w, http.StatusOK, n)
}

// test enqueues a notification — and if a sender is configured, also dispatches
// it synchronously so the operator immediately sees a result.
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	var req EnqueueRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Channel == "" {
		req.Channel = ChannelEmail
	}
	if req.Template == "" {
		req.Template = TemplateBookingCreated
	}
	n, err := h.svc.EnqueueFromRequest(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	// Best-effort synchronous dispatch so the operator can validate the
	// pipeline end-to-end. If sender is nil (no config yet), we leave the
	// row in 'queued' for the worker to pick up.
	if h.sender != nil {
		_ = h.svc.SendNow(r.Context(), n, h.sender)
	}
	respond.Body(w, http.StatusCreated, n)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotificationNotFound):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "notification not found")
	case errors.Is(err, ErrInvalidChannel):
		respond.Error(w, http.StatusBadRequest, "INVALID_CHANNEL", "invalid channel")
	case errors.Is(err, ErrInvalidTemplate):
		respond.Error(w, http.StatusBadRequest, "INVALID_TEMPLATE", "invalid template")
	case errors.Is(err, ErrInvalidRecipient):
		respond.Error(w, http.StatusBadRequest, "INVALID_RECIPIENT", "invalid recipient")
	case errors.Is(err, ErrTemplateRender):
		respond.Error(w, http.StatusUnprocessableEntity, "TEMPLATE_RENDER", "template render failed")
	case errors.Is(err, ErrSenderUnavailable):
		respond.Error(w, http.StatusServiceUnavailable, "SENDER_UNAVAILABLE", "sender unavailable")
	default:
		respond.Error(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
