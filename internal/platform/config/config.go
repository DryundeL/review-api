package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	App      AppConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c DatabaseConfig) DSN() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, c.Port),
		Path:     "/" + c.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(c.SSLMode),
	}
	return u.String()
}

type RedisConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       int
}

func (c RedisConfig) Addr() string {
	return net.JoinHostPort(c.Host, c.Port)
}

type AppConfig struct {
	Env              string
	Port             string
	URL              string
	Version          string
	TelegramBotToken string
	AuthMaxAge       time.Duration
	RateLimitPerMin  int
	CORSOrigins      []string
	OTLPEndpoint     string
	WorkerPoll       time.Duration
	WorkerBatch      int
}

func Load() (*Config, error) {
	if _, err := os.Stat(filepath.Join(".", ".env")); err == nil {
		_ = godotenv.Load()
	}

	redisDB, err := atoiDefault("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}
	rateLimit, err := atoiDefault("RATE_LIMIT_PER_MIN", 60)
	if err != nil {
		return nil, err
	}
	workerBatch, err := atoiDefault("WORKER_BATCH_SIZE", 50)
	if err != nil {
		return nil, err
	}
	authMaxAge, err := durationDefault("AUTH_MAX_AGE", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	workerPoll, err := durationDefault("WORKER_POLL_INTERVAL", 2*time.Second)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   getEnv("DB_NAME", "review"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			User:     os.Getenv("REDIS_USER"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		App: AppConfig{
			Env:              getEnv("APP_ENV", "development"),
			Port:             getEnv("APP_PORT", "8080"),
			URL:              getEnv("APP_URL", "http://localhost:8080"),
			Version:          getEnv("APP_VERSION", "dev"),
			TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
			AuthMaxAge:       authMaxAge,
			RateLimitPerMin:  rateLimit,
			CORSOrigins:      parseCommaSeparatedList(os.Getenv("APP_CORS_ORIGINS")),
			OTLPEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
			WorkerPoll:       workerPoll,
			WorkerBatch:      workerBatch,
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.Database.Host == "" || c.Database.Port == "" || c.Database.DBName == "" {
		return fmt.Errorf("DB_HOST, DB_PORT and DB_NAME are required")
	}
	if c.Redis.Host == "" || c.Redis.Port == "" {
		return fmt.Errorf("REDIS_HOST and REDIS_PORT are required")
	}
	if c.App.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if c.App.RateLimitPerMin < 1 {
		return fmt.Errorf("RATE_LIMIT_PER_MIN must be >= 1")
	}
	if c.App.WorkerBatch < 1 {
		return fmt.Errorf("WORKER_BATCH_SIZE must be >= 1")
	}
	switch c.App.Env {
	case "development", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of: development, staging, production")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoiDefault(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return n, nil
}

func durationDefault(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func parseCommaSeparatedList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
