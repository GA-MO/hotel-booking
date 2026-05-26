package notification

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Enqueue validates the inputs and inserts an outbox row. relatedType / id
// are stored verbatim — domain modules pass e.g. ("booking", booking.ID).
func (s *Service) Enqueue(
	ctx context.Context,
	channel Channel,
	template Template,
	recipient string,
	payload map[string]any,
	relatedType string,
	relatedID *uuid.UUID,
) (*Notification, error) {
	if !channel.Valid() {
		return nil, ErrInvalidChannel
	}
	if !template.Valid() {
		return nil, ErrInvalidTemplate
	}
	if err := validateRecipient(channel, recipient); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	n := Notification{
		Channel:     channel,
		Template:    template,
		Recipient:   recipient,
		Payload:     payload,
		RelatedType: relatedType,
		RelatedID:   relatedID,
	}
	return s.repo.Enqueue(ctx, n)
}

// EnqueueRequest is the JSON-friendly version called by the admin/test endpoint.
func (s *Service) EnqueueFromRequest(ctx context.Context, req EnqueueRequest) (*Notification, error) {
	if !req.Channel.Valid() {
		return nil, ErrInvalidChannel
	}
	if !req.Template.Valid() {
		return nil, ErrInvalidTemplate
	}
	if err := validateRecipient(req.Channel, req.Recipient); err != nil {
		return nil, err
	}
	payload := req.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	n := Notification{
		Channel:     req.Channel,
		Template:    req.Template,
		Recipient:   req.Recipient,
		Payload:     payload,
		RelatedType: req.RelatedType,
		RelatedID:   req.RelatedID,
	}
	if req.SendAfter != nil {
		n.SendAfter = *req.SendAfter
	}
	return s.repo.Enqueue(ctx, n)
}

// DispatchPending claims up to `batch` queued rows and asks the sender to
// deliver each. Returns (sent, failed, error). The worker calls this on
// every tick. Errors from individual sends are recorded on the row, not
// returned — only setup errors bubble up.
func (s *Service) DispatchPending(ctx context.Context, batch int, sender Sender) (sent, failed int, err error) {
	return RunOnce(ctx, s.repo, batch, sender)
}

// Get returns one notification (admin debug).
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Notification, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns the most recent notifications (admin debug).
func (s *Service) List(ctx context.Context, limit int) ([]Notification, error) {
	return s.repo.ListLatest(ctx, limit)
}

// SendNow is a synchronous one-shot send — used by the admin test endpoint
// when the operator wants instant feedback rather than waiting for the worker.
func (s *Service) SendNow(ctx context.Context, n *Notification, sender Sender) error {
	subject, html, text, err := RenderEmail(n.Template, n.Payload)
	if err != nil {
		return err
	}
	return sender.Send(ctx, n.Channel, n.Recipient, subject, html, text)
}

// validateRecipient applies channel-specific format checks.
func validateRecipient(channel Channel, recipient string) error {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return ErrInvalidRecipient
	}
	switch channel {
	case ChannelEmail:
		if _, err := mail.ParseAddress(recipient); err != nil {
			return ErrInvalidRecipient
		}
	case ChannelLINE:
		// LINE user IDs are opaque strings — we just enforce non-empty
		// for now; tightening can come with the real LINE sender.
	case ChannelSMS:
		// E.164 is the eventual target — accept anything non-empty for now.
	}
	return nil
}

// SendAfterFromNow is a small helper for callers building EnqueueRequest with
// a delayed send (e.g., 24h reminder).
func SendAfterFromNow(d time.Duration) *time.Time {
	t := time.Now().Add(d)
	return &t
}
