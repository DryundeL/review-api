package domain

import (
	"regexp"
	"review-api/internal/modules/users/contract"
)

var contentTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

type ContentPreferences struct {
	Types           []string `json:"types"`
	NeedsOnboarding bool     `json:"needsOnboarding"`
}

func ContentTypes(types []string) ([]string, error) {
	if len(types) == 0 || len(types) > 32 {
		return nil, contract.FieldError{Field: "types"}
	}
	seen := make(map[string]bool, len(types))
	for _, code := range types {
		if !contentTypePattern.MatchString(code) || seen[code] {
			return nil, contract.FieldError{Field: "types"}
		}
		seen[code] = true
	}
	return append([]string(nil), types...), nil
}
