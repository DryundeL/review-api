package contract

import "errors"

const EventCreated = "work.created"

var (
	ErrNotFound       = errors.New("work not found")
	ErrAuthorNotFound = errors.New("author not found")
	ErrInvalid        = errors.New("invalid work")
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
