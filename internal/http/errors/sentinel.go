package errors

import "errors"

var (
	ErrUserNotInContext   = errors.New("user not in context")
	ErrInvalidRequestBody = errors.New("invalid request body")
	ErrMethodNotAllowed   = errors.New("method not allowed")
	ErrForbidden          = errors.New("forbidden")
	ErrNotImplemented     = errors.New("not implemented")
	ErrInvalidEmailFormat = errors.New("invalid email format")
	ErrEmailRequired      = errors.New("email is required")
)
