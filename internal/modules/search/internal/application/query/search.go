package query

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/search/contract"
	"review-api/internal/modules/search/internal/application/ports"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 50
	maxQueryRunes      = 100
)

type SearchHandler struct {
	repo ports.Reader
}

func NewSearchHandler(repo ports.Reader) *SearchHandler {
	return &SearchHandler{repo: repo}
}

type SearchInput struct {
	Q        string
	Limit    int
	ViewerID uuid.UUID
}

type SearchResult struct {
	Users   []ports.UserHit
	Reviews []ports.ReviewHit
	Media   []ports.MediaHit
}

func (h *SearchHandler) Handle(ctx context.Context, in SearchInput) (SearchResult, error) {
	if h == nil || h.repo == nil {
		return SearchResult{}, contract.ErrInvalid
	}
	q := NormalizeQuery(in.Q)
	if q == "" {
		return SearchResult{}, contract.FieldError{Field: "q"}
	}
	if utf8.RuneCountInString(q) > maxQueryRunes {
		return SearchResult{}, contract.FieldError{Field: "q"}
	}
	limit := in.Limit
	if limit <= 0 || limit > maxSearchLimit {
		limit = defaultSearchLimit
	}

	users, err := h.repo.SearchUsers(ctx, q, in.ViewerID, limit)
	if err != nil {
		return SearchResult{}, err
	}
	reviews, err := h.repo.SearchReviews(ctx, q, limit)
	if err != nil {
		return SearchResult{}, err
	}
	media, err := h.repo.SearchMedia(ctx, q, limit)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Users: users, Reviews: reviews, Media: media}, nil
}

// NormalizeQuery trims spaces and a leading @ (TG nick as typed).
func NormalizeQuery(raw string) string {
	q := strings.TrimSpace(raw)
	q = strings.TrimPrefix(q, "@")
	return strings.TrimSpace(q)
}
