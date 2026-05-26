package notification

import (
	"time"

	"github.com/google/uuid"
)

// Channel is the delivery transport. Email goes via Resend Phase 1; LINE
// follows in Phase 2 (Thai market is LINE-heavy); SMS is reserved.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelLINE  Channel = "line"
	ChannelSMS   Channel = "sms"
)

func (c Channel) Valid() bool {
	switch c {
	case ChannelEmail, ChannelLINE, ChannelSMS:
		return true
	}
	return false
}

// Template names a renderable bilingual (TH/EN) message body. Templates are
// versioned by name — change the content in templates.go, or add a new name
// if the structure changes incompatibly.
type Template string

const (
	TemplateBookingCreated         Template = "booking_created"
	TemplateBookingConfirmed       Template = "booking_confirmed"
	TemplateBookingCancelled       Template = "booking_cancelled"
	TemplateBookingCheckInReminder Template = "booking_check_in_reminder"
	TemplateBookingPostStay        Template = "booking_post_stay"
)

func (t Template) Valid() bool {
	switch t {
	case TemplateBookingCreated, TemplateBookingConfirmed, TemplateBookingCancelled,
		TemplateBookingCheckInReminder, TemplateBookingPostStay:
		return true
	}
	return false
}

// Status is the queue lifecycle of a single notification row.
type Status string

const (
	StatusQueued  Status = "queued"
	StatusSending Status = "sending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
	StatusDead    Status = "dead"
)

// Notification is one outbox row.
type Notification struct {
	ID          uuid.UUID      `json:"id"`
	Channel     Channel        `json:"channel"`
	Template    Template       `json:"template"`
	Recipient   string         `json:"recipient"`
	Payload     map[string]any `json:"payload"`
	RelatedType string         `json:"related_type,omitempty"`
	RelatedID   *uuid.UUID     `json:"related_id,omitempty"`

	Status    Status `json:"status"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"last_error,omitempty"`

	SendAfter time.Time  `json:"send_after"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// EnqueueRequest is the input shape used by the service Enqueue method
// (and the POST /v1/notifications/test admin endpoint).
type EnqueueRequest struct {
	Channel     Channel        `json:"channel"`
	Template    Template       `json:"template"`
	Recipient   string         `json:"recipient"`
	Payload     map[string]any `json:"payload,omitempty"`
	RelatedType string         `json:"related_type,omitempty"`
	RelatedID   *uuid.UUID     `json:"related_id,omitempty"`
	SendAfter   *time.Time     `json:"send_after,omitempty"`
}

// ListResponse wraps a slice for the admin endpoint.
type ListResponse struct {
	Notifications []Notification `json:"notifications"`
	Total         int            `json:"total"`
}

// MaxAttempts is the retry ceiling; past it, status becomes 'dead'.
const MaxAttempts = 5
