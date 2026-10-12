package ports

import (
	"context"
	"time"
)

// LoginRecord is the short-lived browser sign-in stored until it expires.
type LoginRecord struct {
	Browser    string
	Device     string
	Status     string
	Token      string
	TelegramID int64
}

// LoginStore keeps login challenges. Redis TTL is the expiry.
type LoginStore interface {
	Create(ctx context.Context, id string, rec LoginRecord, ttl time.Duration) error
	Get(ctx context.Context, id string) (LoginRecord, error)
	Resolve(ctx context.Context, id, status string, telegramID int64, ttl time.Duration) error
	SetToken(ctx context.Context, id, token string) (string, error)
}
