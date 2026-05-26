package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/booking"
	"github.com/GA-MO/hotel-booking/apps/api/internal/config"
	"github.com/GA-MO/hotel-booking/apps/api/internal/hotel"
	"github.com/GA-MO/hotel-booking/apps/api/internal/landing"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
	"github.com/GA-MO/hotel-booking/apps/api/internal/pricing"
	"github.com/GA-MO/hotel-booking/apps/api/internal/roomtype"
)

type Server struct {
	cfg      *config.Config
	db       *pgxpool.Pool
	rdb      *redis.Client
	logger   *slog.Logger
	router   *chi.Mux
	jwt      *auth.JWT

	authHdl     *auth.Handler
	hotelHdl    *hotel.Handler
	roomTypeHdl *roomtype.Handler
	landingHdl  *landing.Handler
	pricingHdl  *pricing.Handler
	bookingHdl  *booking.Handler
}

func New(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client, logger *slog.Logger) *Server {
	jwtSvc := auth.NewJWT(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	authSvc := auth.NewService(auth.NewRepository(pool), jwtSvc)
	hotelSvc := hotel.NewService(hotel.NewRepository(pool))
	roomTypeSvc := roomtype.NewService(roomtype.NewRepository(pool))
	landingSvc := landing.NewService(landing.NewRepository(pool))
	pricingSvc := pricing.NewService(pricing.NewRepository(pool))
	bookingSvc := booking.NewService(booking.NewRepository(pool))

	s := &Server{
		cfg: cfg, db: pool, rdb: rdb, logger: logger, jwt: jwtSvc,
		authHdl:     auth.NewHandler(authSvc, jwtSvc),
		hotelHdl:    hotel.NewHandler(hotelSvc, jwtSvc),
		roomTypeHdl: roomtype.NewHandler(roomTypeSvc, jwtSvc),
		landingHdl:  landing.NewHandler(landingSvc, jwtSvc),
		pricingHdl:  pricing.NewHandler(pricingSvc, jwtSvc),
		bookingHdl:  booking.NewHandler(bookingSvc, jwtSvc),
	}
	s.router = s.routes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) routes() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	r.Route("/v1", func(r chi.Router) {
		// Auth (public for signup/login/refresh; me requires bearer token).
		r.Mount("/auth", s.authHdl.Routes())

		// Top-level hotels CRUD (auth-gated inside the handler).
		r.Mount("/hotels", s.hotelHdl.Routes())

		// Per-hotel sub-resources. Auth is applied once here at the parent
		// level so each sub-module avoids duplicating RequireAuth. Modules
		// with multiple top-level paths (roomtype, pricing) AttachTo; others
		// with a single base path (landing, booking) Mount under that path.
		r.Route("/hotels/{hotel_id}", func(rr chi.Router) {
			rr.Use(auth.RequireAuth(s.jwt))
			s.roomTypeHdl.AttachTo(rr) // /room-types, /room-types/{id}/..., /photos
			s.pricingHdl.AttachTo(rr)  // /availability, /pricing-rules
			rr.Mount("/landing", s.landingHdl.Routes())
			rr.Mount("/bookings", s.bookingHdl.Routes())
		})

		// Public (no auth) — used by booking-web for guest checkout + ISR.
		r.Route("/public", func(rr chi.Router) {
			rr.Mount("/landing", s.landingHdl.PublicRoutes())  // /{slug}/{locale}
			rr.Mount("/bookings", s.bookingHdl.PublicRoutes()) // /{reference}
			rr.Mount("/quote", s.pricingHdl.PublicRoutes())    // /{slug}
			rr.Post("/hotels/{slug}/bookings",
				s.bookingHdl.PublicCreateHandler(s.resolveHotelBySlug))
		})

		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			respond.Body(w, http.StatusOK, map[string]string{"pong": "ok"})
		})
	})

	return r
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	respond.Body(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := withTimeout(r.Context(), 3*time.Second)
	defer cancel()

	status := map[string]string{"db": "ok", "redis": "ok"}
	code := http.StatusOK

	if err := s.db.Ping(ctx); err != nil {
		status["db"] = "down: " + err.Error()
		code = http.StatusServiceUnavailable
	}
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		status["redis"] = "down: " + err.Error()
		code = http.StatusServiceUnavailable
	}

	respond.Body(w, code, status)
}

// resolveHotelBySlug is passed into the booking module's public create handler
// so the guest checkout endpoint can map a public slug to an internal hotel
// id. Only hotels with status='live' are eligible for public bookings.
func (s *Server) resolveHotelBySlug(slug string) (uuid.UUID, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id FROM hotels
		WHERE slug = $1 AND status = 'live' AND deleted_at IS NULL
	`, slug).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, false
		}
		s.logger.Warn("resolveHotelBySlug", "err", err)
		return uuid.Nil, false
	}
	return id, true
}
