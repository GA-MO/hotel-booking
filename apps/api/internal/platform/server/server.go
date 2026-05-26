package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/config"
	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/respond"
)

type Server struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	rdb     *redis.Client
	logger  *slog.Logger
	router  *chi.Mux
	authHdl *auth.Handler
}

func New(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client, logger *slog.Logger) *Server {
	jwtSvc := auth.NewJWT(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo, jwtSvc)
	authHdl := auth.NewHandler(authSvc, jwtSvc)

	s := &Server{
		cfg:     cfg,
		db:      pool,
		rdb:     rdb,
		logger:  logger,
		authHdl: authHdl,
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
		r.Mount("/auth", s.authHdl.Routes())

		// Phase 1 domains will mount here:
		//   r.Mount("/hotels", hotel.Routes(...))
		//   r.Mount("/bookings", booking.Routes(...))
		//   r.Mount("/landing", landing.Routes(...))
		//   r.Mount("/subscriptions", billing.Routes(...))

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
