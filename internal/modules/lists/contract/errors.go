package contract

import (
	"errors"
	"strconv"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
	ErrConflict = errors.New("conflict")
	// ErrRevisionConflict: the expected revision is no longer current.
	ErrRevisionConflict = errors.New("revision conflict")
	// ErrModerated: the object or its template is hidden by a moderator.
	ErrModerated = errors.New("hidden by moderator")
	// ErrQuotaExceeded: the author reached the limit of templates or results.
	ErrQuotaExceeded = errors.New("quota exceeded")
	// ErrForbidden: the action is not allowed to this user, e.g. reporting one's own object.
	ErrForbidden = errors.New("forbidden")
	// ErrAlreadyReported: the reporter already has an open report on this object.
	ErrAlreadyReported = errors.New("already reported")
)

// Stable reasons for FieldError. They are part of the API contract.
const (
	ReasonRequired  = "required"
	ReasonTooLong   = "too_long"
	ReasonTooMany   = "too_many"
	ReasonDuplicate = "duplicate"
	ReasonImmutable = "immutable"
	ReasonInvalid   = "invalid"
	ReasonMismatch  = "type_mismatch"
)

// FieldError names the rejected field, an optional index in a collection (-1 if none)
// and a stable reason. All parts are fixed tokens or numbers, never raw input.
type FieldError struct {
	Field  string
	Index  int
	Reason string
}

func Field(field, reason string) FieldError {
	return FieldError{Field: field, Index: -1, Reason: reason}
}

func FieldAt(field string, index int, reason string) FieldError {
	return FieldError{Field: field, Index: index, Reason: reason}
}

func (e FieldError) Error() string {
	if e.Field == "" {
		return ErrInvalid.Error()
	}
	if e.Index >= 0 {
		return "invalid " + e.Field + "[" + strconv.Itoa(e.Index) + "]"
	}
	return "invalid " + e.Field
}

func (e FieldError) Unwrap() error {
	return ErrInvalid
}
