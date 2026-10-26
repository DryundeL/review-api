package query

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"

	"review-api/internal/modules/sitemap/internal/application/ports"
)

var ErrInvalid = errors.New("invalid sitemap request")

type Request struct {
	Entity string
	After  uuid.UUID
	Limit  int
}

type Item struct {
	ID uuid.UUID `json:"id"`
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor"`
}

func ParseRequest(entity, limit, cursor string) (Request, error) {
	req := Request{Entity: entity, Limit: 500}
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return Request{}, ErrInvalid
		}
		req.Limit = n
	}
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil || id == uuid.Nil {
			return Request{}, ErrInvalid
		}
		req.After = id
	}
	if !valid(req) {
		return Request{}, ErrInvalid
	}
	return req, nil
}

func valid(req Request) bool {
	return (req.Entity == "media-objects" || req.Entity == "reviews" || req.Entity == "authors") && req.Limit >= 1 && req.Limit <= 1000
}

type ListHandler struct{ repo ports.Reader }

func NewListHandler(repo ports.Reader) *ListHandler { return &ListHandler{repo: repo} }

func (h *ListHandler) Handle(ctx context.Context, req Request) (Page, error) {
	if !valid(req) {
		return Page{}, ErrInvalid
	}
	ids, err := h.repo.List(ctx, req.Entity, req.After, req.Limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Item, 0, min(len(ids), req.Limit))}
	if len(ids) > req.Limit {
		ids = ids[:req.Limit]
		page.NextCursor = ids[len(ids)-1].String()
	}
	for _, id := range ids {
		page.Items = append(page.Items, Item{ID: id})
	}
	return page, nil
}
