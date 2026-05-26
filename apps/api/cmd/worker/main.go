package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GA-MO/hotel-booking/apps/api/internal/booking"
	"github.com/GA-MO/hotel-booking/apps/api/internal/config"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/db"
)

// Async worker entrypoint. Phase 0 stub — fleshes out in Phase 1 with:
//   - expire pending_payment bookings
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

	slog.Info("worker started", "env", cfg.Env)

	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()

	// Run once on startup so we don't wait a full minute on boot.
	runJobs(ctx, bookingSvc)

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down")
			return
		case <-tick.C:
			runJobs(ctx, bookingSvc)
		}
	}
}

func runJobs(ctx context.Context, bookingSvc *booking.Service) {
	expired, err := bookingSvc.ExpirePending(ctx)
	if err != nil {
		slog.Error("expire pending bookings", "err", err)
	} else if expired > 0 {
		slog.Info("expired pending bookings", "count", expired)
	}
}
