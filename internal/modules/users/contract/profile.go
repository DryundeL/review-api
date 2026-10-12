package contract

import "errors"

// ErrInvalidProfile means the Telegram user cannot be stored.
// Callers should skip the update instead of retrying it.
var (
	ErrInvalidProfile = errors.New("invalid telegram profile")
	ErrNotFound       = errors.New("user not found")
)

// TelegramProfile is a full snapshot of a Telegram user object.
// Empty optional strings clear the stored value on the next EnsureUser.
type TelegramProfile struct {
	TelegramID   int64
	IsBot        bool
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
	IsPremium    bool
}
