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

type GetWorkHandler struct {
	repo ports.Reader
}

func NewGetWorkHandler(repo ports.Reader) *GetWorkHandler {
	return &GetWorkHandler{repo: repo}
}

func (h *GetWorkHandler) Handle(ctx context.Context, id uuid.UUID) (ports.Work, error) {
	if h == nil || h.repo == nil || id == uuid.Nil {
		return ports.Work{}, contract.ErrNotFound
	}
	return h.repo.Get(ctx, id)
}

type SearchWorksHandler struct {
	repo ports.Reader
}

func NewSearchWorksHandler(repo ports.Reader) *SearchWorksHandler {
	return &SearchWorksHandler{repo: repo}
}

func (h *SearchWorksHandler) Handle(ctx context.Context, query, typeCode string, limit int) ([]ports.Work, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) > 100 {
		return nil, contract.FieldError{Field: "q"}
	}
	typeCode = strings.TrimSpace(typeCode)
	if typeCode != "" && !typeCodePattern.MatchString(typeCode) {
		return nil, contract.FieldError{Field: "type"}
	}
	if limit <= 0 || limit > 50 {
		return nil, contract.FieldError{Field: "limit"}
	}
	return h.repo.Search(ctx, query, typeCode, limit)
}
