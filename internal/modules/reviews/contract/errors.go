package contract

import "errors"

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("review type already exists")
	ErrInvalid  = errors.New("invalid review type")
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
