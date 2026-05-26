package notification

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// fakeSender is a minimal in-memory Sender suitable for unit tests — it
// records every call and can be configured to return an error.
type fakeSender struct {
	mu    sync.Mutex
	calls []sendCall
	err   error
}

type sendCall struct {
	Channel   Channel
	Recipient string
	Subject   string
	HTML      string
	Text      string
}

func (f *fakeSender) Send(_ context.Context, ch Channel, rcpt, subject, html, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sendCall{ch, rcpt, subject, html, text})
	return f.err
}

func TestLogSender_AlwaysSucceeds(t *testing.T) {
	t.Parallel()
	s := &LogSender{}
	err := s.Send(context.Background(), ChannelEmail, "x@example.com", "subj", "<p>html</p>", "html")
	if err != nil {
		t.Fatalf("LogSender returned err: %v", err)
	}
}

func TestLineSender_NotImplemented(t *testing.T) {
	t.Parallel()
	err := LineSender{}.Send(context.Background(), ChannelLINE, "U123", "s", "h", "t")
	if err == nil {
		t.Fatalf("expected error from LineSender stub")
	}
}

func TestCompositeSender_Routes(t *testing.T) {
	t.Parallel()
	email := &fakeSender{}
	composite := CompositeSender{Email: email, LINE: LineSender{}}

	if err := composite.Send(context.Background(), ChannelEmail, "x@y.z", "s", "h", "t"); err != nil {
		t.Fatalf("email route: %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("email sender not invoked: %d calls", len(email.calls))
	}

	if err := composite.Send(context.Background(), ChannelSMS, "+66...", "s", "h", "t"); err == nil {
		t.Fatalf("expected SMS to fail (not implemented)")
	}
	if err := composite.Send(context.Background(), Channel("nope"), "x", "s", "h", "t"); err == nil {
		t.Fatalf("expected unknown channel to fail")
	}
}

func TestResendSender_Disabled(t *testing.T) {
	t.Parallel()
	s := &ResendSender{} // no API key
	err := s.Send(context.Background(), ChannelEmail, "x@y.z", "s", "h", "t")
	if err == nil || !errors.Is(err, ErrSenderUnavailable) {
		t.Fatalf("expected ErrSenderUnavailable, got %v", err)
	}
}

func TestResendSender_WrongChannel(t *testing.T) {
	t.Parallel()
	s := &ResendSender{APIKey: "x", From: "y"}
	err := s.Send(context.Background(), ChannelLINE, "u", "s", "h", "t")
	if err == nil || !errors.Is(err, ErrSenderUnavailable) {
		t.Fatalf("expected ErrSenderUnavailable, got %v", err)
	}
}

// --- worker integration with DB ---

func TestRunOnce_MarksSent(t *testing.T) {
	pool := openTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	n, err := repo.Enqueue(ctx, Notification{
		Channel:   ChannelEmail,
		Template:  TemplateBookingConfirmed,
		Recipient: "guest@example.com",
		Payload:   testPayload(),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() { cleanupNotifications(t, pool, n.ID) })

	sender := &fakeSender{}
	sent, failed, err := RunOnce(ctx, repo, 10, sender)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if sent != 1 || failed != 0 {
		t.Fatalf("counts: want sent=1 failed=0, got sent=%d failed=%d", sent, failed)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("sender not called: %d", len(sender.calls))
	}

	got, err := repo.GetByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusSent {
		t.Fatalf("status: want sent, got %s", got.Status)
	}
	if got.SentAt == nil {
		t.Fatalf("sent_at not set")
	}
}

func TestRunOnce_RenderErrorDeadLetters(t *testing.T) {
	pool := openTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Insert with an invalid template name — render will fail.
	n := Notification{
		Channel:   ChannelEmail,
		Template:  Template("does_not_exist"),
		Recipient: "guest@example.com",
		Payload:   testPayload(),
	}
	inserted, err := repo.Enqueue(ctx, n)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() { cleanupNotifications(t, pool, inserted.ID) })

	sender := &fakeSender{}
	_, failed, err := RunOnce(ctx, repo, 10, sender)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if failed != 1 {
		t.Fatalf("failed count: want 1, got %d", failed)
	}
	got, err := repo.GetByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusDead {
		t.Fatalf("status: want dead, got %s", got.Status)
	}
}

// fixture for booking-confirmed template
func testPayload() map[string]any {
	return map[string]any{
		"locale":         "th",
		"reference":      "REF-XYZ",
		"hotel_name":     "Test Hotel",
		"guest_name":     "Test Guest",
		"check_in_date":  "2026-06-01",
		"check_out_date": "2026-06-03",
		"nights":         2,
		"currency":       "THB",
		"total":          "2400.00",
	}
}

// Catch obviously wrong UUIDs that slip into tests.
var _ = uuid.Nil
