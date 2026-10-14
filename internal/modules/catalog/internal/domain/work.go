package domain

import (
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/catalog/contract"
	reviewscontract "review-api/internal/modules/reviews/contract"
)

const (
	maxTitleLen = 200
	maxTypes    = 13
	maxCoverLen = 2048
	minYear     = 1800
	maxYear     = 2100
)

var typeCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// Work is a title that reviews point at: a specific film, book, or series.
type Work struct {
	ID            uuid.UUID
	Title         string
	PrimaryType   string
	Types         []string
	ReleaseYear   *int
	CoverURL      string
	ReleaseStatus string
	Origin        string
	CreatedBy     uuid.UUID
}

type Draft struct {
	Title         string
	PrimaryType   string
	Types         []string
	ReleaseYear   *int
	CoverURL      string
	ReleaseStatus string
}

func NewUserWork(author uuid.UUID, draft Draft) (Work, error) {
	if author == uuid.Nil {
		return Work{}, contract.ErrAuthorNotFound
	}
	title, err := parseTitle(draft.Title)
	if err != nil {
		return Work{}, err
	}
	primary, types, err := parseTypes(draft.PrimaryType, draft.Types)
	if err != nil {
		return Work{}, err
	}
	if draft.ReleaseYear != nil && (*draft.ReleaseYear < minYear || *draft.ReleaseYear > maxYear) {
		return Work{}, contract.FieldError{Field: "releaseYear"}
	}
	cover, err := parseCover(draft.CoverURL)
	if err != nil {
		return Work{}, err
	}
	if !reviewscontract.IsReleaseStatus(draft.ReleaseStatus) {
		return Work{}, contract.FieldError{Field: "releaseStatus"}
	}
	var year *int
	if draft.ReleaseYear != nil {
		value := *draft.ReleaseYear
		year = &value
	}
	return Work{
		ID:            uuid.New(),
		Title:         title,
		PrimaryType:   primary,
		Types:         types,
		ReleaseYear:   year,
		CoverURL:      cover,
		ReleaseStatus: draft.ReleaseStatus,
		Origin:        "user",
		CreatedBy:     author,
	}, nil
}

func parseTitle(value string) (string, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if value == "" || utf8.RuneCountInString(value) > maxTitleLen {
		return "", contract.FieldError{Field: "title"}
	}
	return value, nil
}

func parseTypes(primary string, extra []string) (string, []string, error) {
	primary = strings.TrimSpace(primary)
	if !typeCode.MatchString(primary) {
		return "", nil, contract.FieldError{Field: "primaryType"}
	}
	seen := map[string]struct{}{primary: {}}
	types := []string{primary}
	for _, code := range extra {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if !typeCode.MatchString(code) {
			return "", nil, contract.FieldError{Field: "types"}
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		types = append(types, code)
	}
	if len(types) > maxTypes {
		return "", nil, contract.FieldError{Field: "types"}
	}
	sort.Strings(types[1:])
	return primary, types, nil
}

func parseCover(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxCoverLen {
		return "", contract.FieldError{Field: "coverUrl"}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", contract.FieldError{Field: "coverUrl"}
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost"):
		return "", contract.FieldError{Field: "coverUrl"}
	case strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal"):
		return "", contract.FieldError{Field: "coverUrl"}
	}
	if ip := net.ParseIP(host); ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return "", contract.FieldError{Field: "coverUrl"}
	}
	return parsed.String(), nil
}

func dropControl(r rune) rune {
	if unicode.IsControl(r) {
		return -1
	}
	return r
}
