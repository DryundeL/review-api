package contract

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrAlreadyReported = errors.New("already reported")
	ErrInvalid         = errors.New("invalid")
	ErrForbidden       = errors.New("forbidden")
	// ErrRevisionConflict: the expected revision is no longer current.
	ErrRevisionConflict = errors.New("revision conflict")
)

// FieldError names the rejected field and an optional stable reason. Both are fixed tokens, never raw input.
type FieldError struct {
	Field  string
	Reason string
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
