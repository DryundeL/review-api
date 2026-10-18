package contract

import "errors"

// ErrInvalidProfile means the Telegram user cannot be stored.
// Callers should skip the update instead of retrying it.
var (
	ErrInvalidProfile = errors.New("invalid telegram profile")
	ErrNotFound       = errors.New("user not found")
	ErrInvalid        = errors.New("invalid")
)

// FieldError names the rejected field. The field is a fixed token, never raw input.
type FieldError struct {
	Field string
}

func (e FieldError) Error() string {
	if e.Field == "" {
		return ErrInvalid.Error()
	}
	return "invalid " + e.Field
}

func (e FieldError) Unwrap() error {
	return ErrInvalid
}

// TelegramProfile is a full snapshot of a Telegram user object.
// Empty optional strings clear the stored value on the next EnsureUser,
// except PhotoURL: empty keeps the previously stored Telegram CDN link
// (bot updates never carry photo_url).
type TelegramProfile struct {
	TelegramID   int64
	IsBot        bool
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
	IsPremium    bool
	PhotoURL     string
}
