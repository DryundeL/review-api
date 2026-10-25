package domain

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/catalog/contract"
	reviewscontract "review-api/internal/modules/reviews/contract"
)

const (
	maxTitleLen    = 200
	maxGenreLen    = 100
	maxPosterLen   = 2048
	maxExternalLen = 128
	minYear        = 1800
	maxYear        = 2100
	maxImportBatch = 500
)

var typeCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
var externalToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// Info contains the validated metadata stored in media_objects.info.
type Info struct {
	Genre         string `json:"genre,omitempty"`
	ReleaseYear   *int   `json:"releaseYear,omitempty"`
	ReleaseStatus string `json:"releaseStatus"`
}

// MediaObject is a catalog title that reviews point at.
type MediaObject struct {
	ID             uuid.UUID
	Title          string
	OriginalTitle  string
	Type           string
	Info           Info
	PosterURL      string
	ExternalSource string
	ExternalID     string
	CreatedBy      *uuid.UUID
}

type Draft struct {
	Info           *Info  `json:"info"`
	Title          string `json:"title"`
	OriginalTitle  string `json:"originalTitle"`
	Type           string `json:"type"`
	Genre          string `json:"genre"`
	ReleaseYear    *int   `json:"releaseYear"`
	PosterURL      string `json:"posterUrl"`
	ReleaseStatus  string `json:"releaseStatus"`
	ExternalSource string `json:"externalSource"`
	ExternalID     string `json:"externalId"`
}

func MaxImportBatch() int { return maxImportBatch }

func NewImported(draft Draft) (MediaObject, error) {
	if draft.Info != nil {
		if draft.Genre != "" || draft.ReleaseYear != nil || draft.ReleaseStatus != "" {
			return MediaObject{}, contract.FieldError{Field: "info"}
		}
		draft.Genre = draft.Info.Genre
		draft.ReleaseYear = draft.Info.ReleaseYear
		draft.ReleaseStatus = draft.Info.ReleaseStatus
	}

	title, err := parseTitle(draft.Title)
	if err != nil {
		return MediaObject{}, err
	}
	original, err := parseOptionalTitle(draft.OriginalTitle, "originalTitle")
	if err != nil {
		return MediaObject{}, err
	}
	typ, err := parseType(draft.Type)
	if err != nil {
		return MediaObject{}, err
	}
	genre, err := parseGenre(draft.Genre)
	if err != nil {
		return MediaObject{}, err
	}
	if draft.ReleaseYear != nil && (*draft.ReleaseYear < minYear || *draft.ReleaseYear > maxYear) {
		return MediaObject{}, contract.FieldError{Field: "releaseYear"}
	}
	poster, err := parsePoster(draft.PosterURL)
	if err != nil {
		return MediaObject{}, err
	}
	if !reviewscontract.IsReleaseStatus(draft.ReleaseStatus) {
		return MediaObject{}, contract.FieldError{Field: "releaseStatus"}
	}
	source, id, err := parseExternal(draft.ExternalSource, draft.ExternalID)
	if err != nil {
		return MediaObject{}, err
	}
	var year *int
	if draft.ReleaseYear != nil {
		value := *draft.ReleaseYear
		year = &value
	}
	return MediaObject{
		ID:             uuid.New(),
		Title:          title,
		OriginalTitle:  original,
		Type:           typ,
		Info:           Info{Genre: genre, ReleaseYear: year, ReleaseStatus: draft.ReleaseStatus},
		PosterURL:      poster,
		ExternalSource: source,
		ExternalID:     id,
	}, nil
}

func parseTitle(value string) (string, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if value == "" || utf8.RuneCountInString(value) > maxTitleLen {
		return "", contract.FieldError{Field: "title"}
	}
	return value, nil
}

func parseOptionalTitle(value, field string) (string, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxTitleLen {
		return "", contract.FieldError{Field: field}
	}
	return value, nil
}

func parseGenre(value string) (string, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxGenreLen {
		return "", contract.FieldError{Field: "genre"}
	}
	return value, nil
}

func parseType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !typeCode.MatchString(value) {
		return "", contract.FieldError{Field: "type"}
	}
	return value, nil
}

func parsePoster(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxPosterLen {
		return "", contract.FieldError{Field: "posterUrl"}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", contract.FieldError{Field: "posterUrl"}
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost"):
		return "", contract.FieldError{Field: "posterUrl"}
	case strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal"):
		return "", contract.FieldError{Field: "posterUrl"}
	}
	if ip := net.ParseIP(host); ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return "", contract.FieldError{Field: "posterUrl"}
	}
	return parsed.String(), nil
}

func parseExternal(source, id string) (string, string, error) {
	source = strings.TrimSpace(source)
	id = strings.TrimSpace(id)
	if source == "" && id == "" {
		return "", "", nil
	}
	if source == "" || id == "" {
		return "", "", contract.FieldError{Field: "external"}
	}
	if !externalToken.MatchString(source) || utf8.RuneCountInString(source) > maxExternalLen {
		return "", "", contract.FieldError{Field: "externalSource"}
	}
	if !externalToken.MatchString(id) || utf8.RuneCountInString(id) > maxExternalLen {
		return "", "", contract.FieldError{Field: "externalId"}
	}
	return source, id, nil
}

func dropControl(r rune) rune {
	if unicode.IsControl(r) {
		return -1
	}
	return r
}
