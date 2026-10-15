package query

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/ports"
)

var typeCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

type GetMediaObjectHandler struct {
	repo ports.Reader
}

func NewGetMediaObjectHandler(repo ports.Reader) *GetMediaObjectHandler {
	return &GetMediaObjectHandler{repo: repo}
}

func (h *GetMediaObjectHandler) Handle(ctx context.Context, id uuid.UUID) (ports.MediaObject, error) {
	if h == nil || h.repo == nil || id == uuid.Nil {
		return ports.MediaObject{}, contract.ErrNotFound
	}
	return h.repo.Get(ctx, id)
}

type SearchMediaObjectsHandler struct {
	repo ports.Reader
}

func NewSearchMediaObjectsHandler(repo ports.Reader) *SearchMediaObjectsHandler {
	return &SearchMediaObjectsHandler{repo: repo}
}

func (h *SearchMediaObjectsHandler) Handle(ctx context.Context, query, typeCode string, limit int) ([]ports.MediaObject, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) > 100 {
		return nil, contract.FieldError{Field: "q", Index: -1}
	}
	typeCode = strings.TrimSpace(typeCode)
	if typeCode != "" && !typeCodePattern.MatchString(typeCode) {
		return nil, contract.FieldError{Field: "type", Index: -1}
	}
	if limit <= 0 || limit > 50 {
		return nil, contract.FieldError{Field: "limit", Index: -1}
	}
	return h.repo.Search(ctx, query, typeCode, limit)
}
