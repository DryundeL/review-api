package config

import (
	"strings"
	"testing"
)

func TestDatabaseDSNEscapesPassword(t *testing.T) {
	t.Parallel()
	cfg := DatabaseConfig{
		Host:     "postgres",
		Port:     "5433",
		User:     "postgres",
		Password: "p@ss/word",
		DBName:   "analytic",
		SSLMode:  "disable",
	}
	got := cfg.DSN()
	want := "postgres://postgres:p%40ss%2Fword@postgres:5433/analytic?sslmode=disable"
	if got != want {
		t.Fatalf("DSN = %s, want %s", got, want)
	}
}

func TestRedisAddr(t *testing.T) {
	t.Parallel()
	cfg := RedisConfig{Host: "redis", Port: "6379"}
	if got := cfg.Addr(); got != "redis:6379" {
		t.Fatalf("Addr = %s", got)
	}
}

func TestValidateRequiresSecrets(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Database: DatabaseConfig{Host: "h", Port: "5432", DBName: "db"},
		Redis:    RedisConfig{Host: "r", Port: "6379"},
		App: AppConfig{
			Env:                 "development",
			RateLimitPerMin:     60,
			WorkerBatch:         50,
			MediaStorageDir:     "storage",
			MediaPublicPrefix:   "/storage",
			MediaUploadsPerHour: 120,
			ListsTemplateQuota:  50,
			ListsResultQuota:    200,
			ListsReportsPerHour: 20,
			ReportHideThreshold: 10,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
	cfg.Database.Password = "root"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected telegram token error")
	}
	cfg.App.TelegramBotToken = "tok"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected jwt and bot username error")
	}
	cfg.App.JWTSecret = strings.Repeat("s", 32)
	cfg.App.TelegramBotUsername = "review_bot"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected admin token error")
	}
	cfg.App.AdminToken = strings.Repeat("a", 32)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.App.AdminToken = cfg.App.JWTSecret
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected admin token to differ from jwt")
	}
}

func TestValidateReportHideThreshold(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Database: DatabaseConfig{Host: "h", Port: "5432", DBName: "db", Password: "root"},
		Redis:    RedisConfig{Host: "r", Port: "6379"},
		App: AppConfig{
			Env:                 "development",
			TelegramBotToken:    "tok",
			TelegramBotUsername: "review_bot",
			JWTSecret:           strings.Repeat("s", 32),
			AdminToken:          strings.Repeat("a", 32),
			RateLimitPerMin:     60,
			WorkerBatch:         50,
			MediaStorageDir:     "storage",
			MediaPublicPrefix:   "/storage",
			MediaUploadsPerHour: 120,
			ListsTemplateQuota:  50,
			ListsResultQuota:    200,
			ListsReportsPerHour: 20,
			ReportHideThreshold: 0,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected REPORT_HIDE_THRESHOLD error")
	}
	cfg.App.ReportHideThreshold = 10
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMedia(t *testing.T) {
	t.Parallel()
	valid := func() *Config {
		return &Config{
			Database: DatabaseConfig{Host: "h", Port: "5432", DBName: "d", Password: "p"},
			Redis:    RedisConfig{Host: "h", Port: "6379"},
			App: AppConfig{
				Env: "development", TelegramBotToken: "tok", TelegramBotUsername: "review_bot",
				JWTSecret: strings.Repeat("s", 32), AdminToken: strings.Repeat("a", 32),
				RateLimitPerMin: 60, WorkerBatch: 50, ReportHideThreshold: 10,
				MediaStorageDir: "storage", MediaPublicPrefix: "/storage", MediaUploadsPerHour: 120,
				ListsTemplateQuota: 50, ListsResultQuota: 200, ListsReportsPerHour: 20,
			},
		}
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Config){
		"empty dir":           func(c *Config) { c.App.MediaStorageDir = "" },
		"http prefix":         func(c *Config) { c.App.MediaPublicPrefix = "http://cdn.example" },
		"relative prefix":     func(c *Config) { c.App.MediaPublicPrefix = "storage" },
		"zero upload quota":   func(c *Config) { c.App.MediaUploadsPerHour = 0 },
		"zero template quota": func(c *Config) { c.App.ListsTemplateQuota = 0 },
		"zero result quota":   func(c *Config) { c.App.ListsResultQuota = 0 },
		"zero report limit":   func(c *Config) { c.App.ListsReportsPerHour = 0 },
	} {
		cfg := valid()
		mutate(cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}
