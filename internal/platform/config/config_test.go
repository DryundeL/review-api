package config

import (
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
		App:      AppConfig{Env: "development", RateLimitPerMin: 60, WorkerBatch: 50},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
	cfg.Database.Password = "root"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected telegram token error")
	}
	cfg.App.TelegramBotToken = "tok"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
