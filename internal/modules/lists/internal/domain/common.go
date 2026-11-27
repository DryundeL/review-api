// Package domain holds the rules of list templates and results from SPEC-lists.md.
// It validates content and state transitions; persistence assigns revisions.
package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"review-api/internal/modules/lists/contract"
)

// Limits from SPEC-lists. Lengths are counted in Unicode code points after trimming.
const (
	MaxTitleLen       = 150
	MaxDescriptionLen = 2000
	MaxItemTitleLen   = 150
	MaxTierNameLen    = 30
	MaxItems          = 50
	MinTiers          = 1
	MaxTiers          = 10
)

type Format string

const (
	FormatTiers      Format = "tiers"
	FormatRanked     Format = "ranked"
	FormatCollection Format = "collection"
)

// ParseFormat returns FormatTiers for an empty value: tiers are the default format.
func ParseFormat(raw string) (Format, error) {
	switch f := Format(strings.TrimSpace(raw)); f {
	case "":
		return FormatTiers, nil
	case FormatTiers, FormatRanked, FormatCollection:
		return f, nil
	default:
		return "", contract.Field("format", contract.ReasonInvalid)
	}
}

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
)

// ParseVisibility returns VisibilityPublic for an empty value: public is the default on publish.
func ParseVisibility(raw string) (Visibility, error) {
	switch v := Visibility(strings.TrimSpace(raw)); v {
	case "":
		return VisibilityPublic, nil
	case VisibilityPublic, VisibilityUnlisted:
		return v, nil
	default:
		return "", contract.Field("visibility", contract.ReasonInvalid)
	}
}

// cleanText trims, rejects control characters (newlines allowed when multiline) and checks length.
func cleanText(raw string, maxLen int, multiline bool) (string, string) {
	s := strings.TrimSpace(raw)
	if !utf8.ValidString(s) {
		return "", contract.ReasonInvalid
	}
	for _, r := range s {
		if multiline && (r == '\n' || r == '\t') {
			continue
		}
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return "", contract.ReasonInvalid
		}
	}
	if utf8.RuneCountInString(s) > maxLen {
		return "", contract.ReasonTooLong
	}
	return s, ""
}

func cleanField(raw string, maxLen int, multiline bool, field string, index int) (string, error) {
	s, reason := cleanText(raw, maxLen, multiline)
	if reason != "" {
		return "", contract.FieldAt(field, index, reason)
	}
	return s, nil
}
