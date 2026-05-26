package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Sender abstracts the channel-specific transport (Resend, LINE, ...). The
// worker calls Send for each claimed row and converts a non-nil error into
// a retry via MarkFailed.
type Sender interface {
	Send(ctx context.Context, channel Channel, recipient, subject, html, text string) error
}

// LogSender records would-be sends to slog and returns nil. Used in dev /
// tests when RESEND_API_KEY is empty, so the queue still drains.
type LogSender struct {
	Logger *slog.Logger
}

func (l *LogSender) Send(_ context.Context, channel Channel, recipient, subject, html, _ string) error {
	logger := l.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("notification.LogSender",
		"channel", string(channel),
		"recipient", recipient,
		"subject", subject,
		"body_len", len(html),
	)
	return nil
}

// ResendSender posts to https://api.resend.com/emails. Stdlib net/http only —
// no SDK dependency. Resend returns 200/201 with `{"id":"..."}` on success
// and non-2xx + JSON body on failure.
type ResendSender struct {
	APIKey   string
	From     string
	BaseURL  string // overridable for tests; defaults to Resend production URL
	HTTPDoer interface {
		Do(req *http.Request) (*http.Response, error)
	}
}

const resendEndpoint = "https://api.resend.com/emails"

func (s *ResendSender) Send(ctx context.Context, channel Channel, recipient, subject, html, text string) error {
	if channel != ChannelEmail {
		return fmt.Errorf("%w: ResendSender only handles email, got %s", ErrSenderUnavailable, channel)
	}
	if s.APIKey == "" || s.From == "" {
		return fmt.Errorf("%w: missing Resend API key or from address", ErrSenderUnavailable)
	}

	endpoint := s.BaseURL
	if endpoint == "" {
		endpoint = resendEndpoint
	}

	body := map[string]any{
		"from":    s.From,
		"to":      []string{recipient},
		"subject": subject,
		"html":    html,
	}
	if text != "" {
		body["text"] = text
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode resend body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("new resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	doer := s.HTTPDoer
	if doer == nil {
		doer = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("resend post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Drain the body so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	// Surface the provider's error body up to the caller — it ends up in
	// notifications.last_error so operators can debug from the admin list.
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("resend status %d: %s", resp.StatusCode, string(bytes.TrimSpace(bodyBytes)))
}

// LineSender is a placeholder so the Sender interface covers LINE — actually
// shipping LINE messages is Phase 2 work.
type LineSender struct{}

func (LineSender) Send(_ context.Context, _ Channel, _, _, _, _ string) error {
	return errors.New("line sender not yet implemented")
}

// CompositeSender routes by channel — wraps an email sender (+ optional LINE).
// This is the type cmd/worker actually instantiates so we can plug Resend or
// Log behind email while LINE remains a stub.
type CompositeSender struct {
	Email Sender
	LINE  Sender
}

func (c CompositeSender) Send(ctx context.Context, channel Channel, recipient, subject, html, text string) error {
	switch channel {
	case ChannelEmail:
		if c.Email == nil {
			return fmt.Errorf("%w: no email sender configured", ErrSenderUnavailable)
		}
		return c.Email.Send(ctx, channel, recipient, subject, html, text)
	case ChannelLINE:
		if c.LINE == nil {
			return fmt.Errorf("%w: no LINE sender configured", ErrSenderUnavailable)
		}
		return c.LINE.Send(ctx, channel, recipient, subject, html, text)
	case ChannelSMS:
		return fmt.Errorf("%w: SMS not yet implemented", ErrSenderUnavailable)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidChannel, channel)
	}
}
