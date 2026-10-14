package domain

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"review-api/internal/modules/reviews/contract"
)

const (
	maxCodeLen  = 32
	maxLabelLen = 64
	maxSort     = 10000
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// ReviewType is one row of review_types. Code does not change after insert.
type ReviewType struct {
	Code      string
	Label     string
	SortOrder int
	IsActive  bool
}

func NewReviewType(code, label string, sortOrder int, active bool) (ReviewType, error) {
	code, err := ParseCode(code)
	if err != nil {
		return ReviewType{}, contract.FieldError{Field: "code"}
	}
	label, err = ParseLabel(label)
	if err != nil {
		return ReviewType{}, err
	}
	if err := ParseSort(sortOrder); err != nil {
		return ReviewType{}, err
	}
	return ReviewType{Code: code, Label: label, SortOrder: sortOrder, IsActive: active}, nil
}

func ParseCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxCodeLen || !codePattern.MatchString(value) {
		return "", contract.FieldError{Field: "code"}
	}
	return value, nil
}

func ParseLabel(value string) (string, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if value == "" || utf8.RuneCountInString(value) > maxLabelLen {
		return "", contract.FieldError{Field: "label"}
	}
	return value, nil
}

func ParseSort(value int) error {
	if value < 0 || value > maxSort {
		return contract.FieldError{Field: "sortOrder"}
	}
	return nil
}

func dropControl(r rune) rune {
	if unicode.IsControl(r) {
		return -1
	}
	return r
}
