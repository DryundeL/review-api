package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/search/contract"
	"review-api/internal/modules/search/internal/application/ports"
)

const (
	defaultTrendLimit = 10
	maxTrendLimit     = 20
)

type TrendsHandler struct {
	repo ports.Reader
}

func NewTrendsHandler(repo ports.Reader) *TrendsHandler {
	return &TrendsHandler{repo: repo}
}

type TrendsInput struct {
	Tab      string
	Limit    int
	ViewerID uuid.UUID
}

type TrendsResult struct {
	Tab     string
	Media   []ports.MediaTrend
	Authors []ports.AuthorTrend
	Types   []ports.TypeTrend
}

func (h *TrendsHandler) Handle(ctx context.Context, in TrendsInput) (TrendsResult, error) {
	if h == nil || h.repo == nil {
		return TrendsResult{}, contract.ErrInvalid
	}
	tab, err := parseTab(in.Tab)
	if err != nil {
		return TrendsResult{}, err
	}
	limit := in.Limit
	if limit <= 0 || limit > maxTrendLimit {
		limit = defaultTrendLimit
	}

	out := TrendsResult{Tab: tab}
	switch tab {
	case ports.TabMedia:
		items, err := h.repo.TopMedia(ctx, limit)
		if err != nil {
			return TrendsResult{}, err
		}
		out.Media = items
	case ports.TabAuthors:
		items, err := h.repo.TopAuthors(ctx, in.ViewerID, limit)
		if err != nil {
			return TrendsResult{}, err
		}
		out.Authors = items
	case ports.TabTypes:
		items, err := h.repo.TopTypes(ctx)
		if err != nil {
			return TrendsResult{}, err
		}
		out.Types = items
	}
	return out, nil
}

func parseTab(raw string) (string, error) {
	if raw == "" {
		return ports.TabMedia, nil
	}
	switch raw {
	case ports.TabMedia, ports.TabAuthors, ports.TabTypes:
		return raw, nil
	default:
		return "", contract.FieldError{Field: "tab"}
	}
}
