package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Env      string `envconfig:"ENV" default:"development"`
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`

	APIPort     int      `envconfig:"API_PORT" default:"8080"`
	CORSOrigins []string `envconfig:"API_CORS_ORIGINS" default:"http://localhost:3000,http://localhost:3001"`

	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	RedisURL    string `envconfig:"REDIS_URL" required:"true"`

	StorageEndpoint      string `envconfig:"STORAGE_ENDPOINT"`
	StorageRegion        string `envconfig:"STORAGE_REGION" default:"us-east-1"`
	StorageAccessKey     string `envconfig:"STORAGE_ACCESS_KEY"`
	StorageSecretKey     string `envconfig:"STORAGE_SECRET_KEY"`
	StorageBucket        string `envconfig:"STORAGE_BUCKET" default:"hotel-booking"`
	StoragePublicBaseURL string `envconfig:"STORAGE_PUBLIC_BASE_URL"`

	ImgproxyBaseURL string `envconfig:"IMGPROXY_BASE_URL"`
	ImgproxyKey     string `envconfig:"IMGPROXY_KEY"`
	ImgproxySalt    string `envconfig:"IMGPROXY_SALT"`

	JWTSecret     string        `envconfig:"JWT_SECRET" required:"true"`
	JWTAccessTTL  time.Duration `envconfig:"JWT_ACCESS_TTL" default:"15m"`
	JWTRefreshTTL time.Duration `envconfig:"JWT_REFRESH_TTL" default:"720h"`

	ResendAPIKey string `envconfig:"RESEND_API_KEY"`
	EmailFrom    string `envconfig:"EMAIL_FROM" default:"no-reply@example.com"`

	LineChannelAccessToken string `envconfig:"LINE_CHANNEL_ACCESS_TOKEN"`
	LineChannelSecret      string `envconfig:"LINE_CHANNEL_SECRET"`

	StripeSecretKey     string `envconfig:"STRIPE_SECRET_KEY"`
	StripeWebhookSecret string `envconfig:"STRIPE_WEBHOOK_SECRET"`
	OmiseSecretKey      string `envconfig:"OMISE_SECRET_KEY"`
	OmisePublicKey      string `envconfig:"OMISE_PUBLIC_KEY"`
}

func Load() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("envconfig: %w", err)
	}
	return &c, nil
}
