package contract

import "github.com/google/uuid"

// UserRef is a minimal public snapshot for other BCs (notifications, etc.).
type UserRef struct {
	ID         uuid.UUID
	TelegramID int64
	Username   string
	FirstName  string
}

// DisplayName prefers @username, otherwise first name.
func (u UserRef) DisplayName() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	return "user"
}
