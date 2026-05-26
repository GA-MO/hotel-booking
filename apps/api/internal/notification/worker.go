package notification

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// RunOnce claims one batch of queued notifications and dispatches each via
// the supplied Sender. Returns (sent, failed, setupErr) — setupErr is only
// returned when claiming the batch itself fails; per-row send failures are
// recorded on the row and counted in `failed`.
//
// Intended to be called on every worker tick (see cmd/worker/main.go) — the
// worker decides the cadence; this function is one cycle.
func RunOnce(ctx context.Context, repo *Repository, batch int, sender Sender) (sent, failed int, err error) {
	if sender == nil {
		return 0, 0, errors.New("notification.RunOnce: nil sender")
	}
	rows, err := repo.ClaimBatch(ctx, batch, time.Now())
	if err != nil {
		return 0, 0, err
	}
	for i := range rows {
		n := &rows[i]
		if dispatchOne(ctx, repo, n, sender) {
			sent++
		} else {
			failed++
		}
	}
	return sent, failed, nil
}

func dispatchOne(ctx context.Context, repo *Repository, n *Notification, sender Sender) bool {
	subject, html, text, rerr := RenderEmail(n.Template, n.Payload)
	if rerr != nil {
		// Bad template / bad payload → dead-letter immediately; retrying
		// won't help.
		if merr := repo.MarkFailed(ctx, n.ID, rerr.Error(), true); merr != nil {
			slog.Error("notification.mark_failed (render)", "id", n.ID, "err", merr)
		}
		return false
	}
	if serr := sender.Send(ctx, n.Channel, n.Recipient, subject, html, text); serr != nil {
		if merr := repo.MarkFailed(ctx, n.ID, serr.Error(), false); merr != nil {
			slog.Error("notification.mark_failed (send)", "id", n.ID, "err", merr)
		}
		return false
	}
	if merr := repo.MarkSent(ctx, n.ID); merr != nil {
		slog.Error("notification.mark_sent", "id", n.ID, "err", merr)
		return false
	}
	return true
}
