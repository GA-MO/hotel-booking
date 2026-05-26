package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/GA-MO/hotel-booking/apps/api/internal/booking"
	"github.com/GA-MO/hotel-booking/apps/api/internal/config"
	"github.com/GA-MO/hotel-booking/apps/api/internal/notification"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/db"
	"github.com/GA-MO/hotel-booking/apps/api/internal/subscription"
)

// Async worker entrypoint. Phase 0 stub — fleshes out in Phase 1 with:
//   - expire pending_payment bookings
//   - drive subscription state transitions (trial_ending → trial_lapsed → ...)
//   - send queued notifications
//   - reconcile subscription billing webhooks
//   - generate invoice PDFs
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	bookingSvc := booking.NewService(booking.NewRepository(pool))
	notificationSvc := notification.NewService(notification.NewRepository(pool))
	subscriptionSvc := newSubscriptionService(pool)
	sender := buildSender(cfg)

	slog.Info("worker started", "env", cfg.Env, "sender", senderName(cfg))

	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()

	// Run once on startup so we don't wait a full minute on boot.
	runJobs(ctx, bookingSvc, notificationSvc, subscriptionSvc, sender)

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down")
			return
		case <-tick.C:
			runJobs(ctx, bookingSvc, notificationSvc, subscriptionSvc, sender)
		}
	}
}

func runJobs(
	ctx context.Context,
	bookingSvc *booking.Service,
	notificationSvc *notification.Service,
	subscriptionSvc *subscription.Service,
	sender notification.Sender,
) {
	expired, err := bookingSvc.ExpirePending(ctx)
	if err != nil {
		slog.Error("expire pending bookings", "err", err)
	} else if expired > 0 {
		slog.Info("expired pending bookings", "count", expired)
	}

	sent, failed, err := notificationSvc.DispatchPending(ctx, 50, sender)
	if err != nil {
		slog.Error("dispatch notifications", "err", err)
	} else if sent > 0 || failed > 0 {
		slog.Info("dispatched notifications", "sent", sent, "failed", failed)
	}

	if subscriptionSvc != nil {
		sum := subscriptionSvc.RunMaintenance(ctx)
		if sum.TrialEnding+sum.TrialLapsed+sum.Suspended+sum.Cancelled+sum.Terminated+sum.ErrorCount > 0 {
			slog.Info("subscription maintenance",
				"trial_ending", sum.TrialEnding,
				"trial_lapsed", sum.TrialLapsed,
				"suspended", sum.Suspended,
				"cancelled", sum.Cancelled,
				"terminated", sum.Terminated,
				"errors", sum.ErrorCount,
			)
		}
	}
}

// newSubscriptionService constructs the subscription service. Wrapped to
// guard the maintenance loop: if the service ever fails to initialise (panic
// during construction), the worker continues running the other jobs.
func newSubscriptionService(pool *pgxpool.Pool) *subscription.Service {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("subscription service unavailable; maintenance scan disabled", "panic", r)
		}
	}()
	return subscription.NewService(subscription.NewRepository(pool))
}

// buildSender picks the right sender based on config:
//   - RESEND_API_KEY empty → LogSender (dev / no-op)
//   - otherwise            → ResendSender for email
// LINE always uses the stub (Phase 2 work).
func buildSender(cfg *config.Config) notification.Sender {
	var email notification.Sender
	if cfg.ResendAPIKey == "" {
		email = &notification.LogSender{}
	} else {
		email = &notification.ResendSender{
			APIKey: cfg.ResendAPIKey,
			From:   cfg.EmailFrom,
		}
	}
	return notification.CompositeSender{
		Email: email,
		LINE:  notification.LineSender{},
	}
}

func senderName(cfg *config.Config) string {
	if cfg.ResendAPIKey == "" {
		return "log"
	}
	return "resend"
}
