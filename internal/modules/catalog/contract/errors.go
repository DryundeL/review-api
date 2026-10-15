package contract

import "errors"

const EventCreated = "media_object.created"

var (
	ErrNotFound = errors.New("media object not found")
	ErrInvalid  = errors.New("invalid media object")
)

// FieldError names the rejected field. The field is a fixed token, never raw input.
type FieldError struct {
	Field string
	Index int // set for batch import; -1 when not applicable
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
