package errors

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	apperrors "review-api/internal/pkg/errors"
)

type Mapped struct {
	Status  int
	Code    string
	Message string
}

func Map(err error, logger *slog.Logger) Mapped {
	if err == nil {
		return Mapped{Status: http.StatusOK}
	}
	if logger == nil {
		logger = slog.Default()
	}

	switch {
	case errors.Is(err, ErrUserNotInContext):
		return Mapped{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "unauthorized"}
	case errors.Is(err, apperrors.ErrNotFound), errors.Is(err, apperrors.ErrUserNotFound):
		return Mapped{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: err.Error()}
	case errors.Is(err, ErrForbidden):
		return Mapped{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: "forbidden"}
	case errors.Is(err, ErrInvalidRequestBody), errors.Is(err, ErrInvalidEmailFormat), errors.Is(err, ErrEmailRequired):
		return Mapped{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: err.Error()}
	case errors.Is(err, ErrMethodNotAllowed):
		return Mapped{Status: http.StatusMethodNotAllowed, Code: "METHOD_NOT_ALLOWED", Message: "method not allowed"}
	case errors.Is(err, ErrNotImplemented):
		return Mapped{Status: http.StatusNotImplemented, Code: "NOT_IMPLEMENTED", Message: "not implemented"}
	}

	if strings.Contains(err.Error(), "validation failed") {
		return Mapped{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: err.Error()}
	}

	logger.Error("unhandled error", "error", err)
	return Mapped{Status: http.StatusInternalServerError, Code: "INTERNAL", Message: "internal server error"}
}
