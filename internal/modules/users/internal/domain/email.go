package domain

import (
	"net/mail"
	"strings"

	"review-api/internal/modules/users/contract"
)

const maxEmailLen = 254

func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 3 || len(value) > maxEmailLen || strings.Count(value, "@") != 1 {
		return "", contract.FieldError{Field: "email"}
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", contract.FieldError{Field: "email"}
	}
	return value, nil
}
