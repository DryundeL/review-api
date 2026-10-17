package query

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/feed/contract"
	"review-api/internal/modules/feed/internal/application/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

// Page is one feed response page.
type Page struct {
	Items      []ports.Item
	NextCursor string
}

type ListFeedHandler struct {
	repo ports.Reader
}

func NewListFeedHandler(repo ports.Reader) *ListFeedHandler {
	return &ListFeedHandler{repo: repo}
}

type ListInput struct {
	Sort     string
	Mode     string
	Type     string
	Cursor   string
	Limit    int
	ViewerID uuid.UUID
}

func (h *ListFeedHandler) Handle(ctx context.Context, in ListInput) (Page, error) {
	if h == nil || h.repo == nil {
		return Page{}, contract.ErrInvalid
	}
	sort, err := parseSort(in.Sort)
	if err != nil {
		return Page{}, err
	}
	mode, err := parseMode(in.Mode)
	if err != nil {
		return Page{}, err
	}
	if mode == ports.ModeFollowing && in.ViewerID == uuid.Nil {
		return Page{}, contract.ErrUnauthorized
	}
	mediaType := strings.TrimSpace(in.Type)
	cur, err := DecodeCursor(in.Cursor, sort)
	if err != nil {
		return Page{}, err
	}
	limit := in.Limit
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	rows, err := h.repo.List(ctx, ports.ListParams{
		Sort:     sort,
		Mode:     mode,
		Type:     mediaType,
		ViewerID: in.ViewerID,
		Cursor:   cur,
		Limit:    limit + 1,
	})
	if err != nil {
		return Page{}, err
	}

	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		encoded, err := EncodeCursor(cursorFromItem(sort, rows[len(rows)-1]))
		if err != nil {
			return Page{}, err
		}
		next = encoded
	}
	return Page{Items: rows, NextCursor: next}, nil
}

func parseSort(raw string) (string, error) {
	if raw == "" {
		return ports.SortNew, nil
	}
	switch raw {
	case ports.SortNew, ports.SortViews, ports.SortLikes:
		return raw, nil
	default:
		return "", contract.FieldError{Field: "sort"}
	}
}

func parseMode(raw string) (string, error) {
	if raw == "" {
		return ports.ModeAll, nil
	}
	switch raw {
	case ports.ModeAll, ports.ModeFollowing:
		return raw, nil
	default:
		return "", contract.FieldError{Field: "mode"}
	}
}
