package contract

import "errors"

const (
	LoginPending   = "pending"
	LoginConfirmed = "confirmed"
	LoginDenied    = "denied"
)

var (
	ErrLoginNotFound = errors.New("login not found")
	ErrLoginClosed   = errors.New("login closed")
)

// Login is the browser sign-in challenge the bot shows to the user.
type Login struct {
	Browser    string
	Device     string
	Status     string
	TelegramID int64
}

// ValidLoginID reports whether id is a generated login challenge id.
func ValidLoginID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
