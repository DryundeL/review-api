package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 50
	MaxSearchLen    = 100
	maxTypeCodeLen  = 64
	maxCursorLen    = 256
)

// ListInput is the raw query of a section or profile tab. Empty values mean "no filter".
type ListInput struct {
	Search string
	Type   string
	Format string
	Cursor string
	Limit  int // 0: DefaultPageSize
}

// MineInput is the raw query of the author's own list.
type MineInput struct {
	Status string
	Cursor string
	Limit  int
}

type ListPage struct {
	Items      []ports.ListCard
	NextCursor string // empty on the last page
}

type ListHandler struct {
	repo ports.ListReader
}

func NewListHandler(repo ports.ListReader) *ListHandler {
	return &ListHandler{repo: repo}
}

type entity string

const (
	entityTemplates entity = "templates"
	entityResults   entity = "results"
)

// Templates is the "Шаблоны" tab: published public templates, newest first publication first.
func (h *ListHandler) Templates(ctx context.Context, in ListInput) (ListPage, error) {
	return h.public(ctx, entityTemplates, uuid.Nil, in)
}

// Results is the "Топы пользователей" tab.
func (h *ListHandler) Results(ctx context.Context, in ListInput) (ListPage, error) {
	return h.public(ctx, entityResults, uuid.Nil, in)
}

// ProfileTemplates lists the public templates of one author. Profile privacy is enforced by the caller.
func (h *ListHandler) ProfileTemplates(ctx context.Context, authorID uuid.UUID, in ListInput) (ListPage, error) {
	if authorID == uuid.Nil {
		return ListPage{}, contract.ErrNotFound
	}
	return h.public(ctx, entityTemplates, authorID, in)
}

func (h *ListHandler) ProfileResults(ctx context.Context, authorID uuid.UUID, in ListInput) (ListPage, error) {
	if authorID == uuid.Nil {
		return ListPage{}, contract.ErrNotFound
	}
	return h.public(ctx, entityResults, authorID, in)
}

// MyTemplates is the author's private list, drafts and unlisted included, by last update.
func (h *ListHandler) MyTemplates(ctx context.Context, authorID uuid.UUID, in MineInput) (ListPage, error) {
	return h.mine(ctx, entityTemplates, authorID, in)
}

func (h *ListHandler) MyResults(ctx context.Context, authorID uuid.UUID, in MineInput) (ListPage, error) {
	return h.mine(ctx, entityResults, authorID, in)
}

func (h *ListHandler) public(ctx context.Context, e entity, authorID uuid.UUID, in ListInput) (ListPage, error) {
	scope := "public"
	if authorID != uuid.Nil {
		scope = "profile"
	}
	q := ports.ListQuery{AuthorID: authorID}
	var err error
	if q.Search, err = parseSearch(in.Search); err != nil {
		return ListPage{}, err
	}
	if q.TypeCode, err = parseTypeCode(in.Type); err != nil {
		return ListPage{}, err
	}
	if strings.TrimSpace(in.Format) != "" {
		if q.Format, err = domain.ParseFormat(in.Format); err != nil {
			return ListPage{}, err
		}
	}
	return h.page(ctx, e, scope, q, in.Cursor, in.Limit)
}

func (h *ListHandler) mine(ctx context.Context, e entity, authorID uuid.UUID, in MineInput) (ListPage, error) {
	if authorID == uuid.Nil {
		return ListPage{}, contract.ErrInvalid
	}
	status := ports.MineStatus(strings.TrimSpace(in.Status))
	switch status {
	case ports.MineAll, ports.MineDraft, ports.MinePublished, ports.MineUnlisted, ports.MineHidden:
	default:
		return ListPage{}, contract.Field("status", contract.ReasonInvalid)
	}
	q := ports.ListQuery{Mine: true, AuthorID: authorID, Status: status}
	// The status is part of the scope: a cursor of one filter is not valid for another.
	return h.page(ctx, e, "mine:"+string(status), q, in.Cursor, in.Limit)
}

// page fetches one row more than asked to know whether a next page exists.
func (h *ListHandler) page(ctx context.Context, e entity, scope string, q ports.ListQuery, rawCursor string, limit int) (ListPage, error) {
	if h == nil || h.repo == nil {
		return ListPage{}, contract.ErrInvalid
	}
	switch {
	case limit == 0:
		limit = DefaultPageSize
	case limit < 1 || limit > MaxPageSize:
		return ListPage{}, contract.Field("limit", contract.ReasonInvalid)
	}
	kind := string(e) + ":" + scope
	after, err := decodeCursor(rawCursor, kind)
	if err != nil {
		return ListPage{}, err
	}
	q.After, q.Limit = after, limit+1
	var cards []ports.ListCard
	if e == entityTemplates {
		cards, err = h.repo.ListTemplates(ctx, q)
	} else {
		cards, err = h.repo.ListResults(ctx, q)
	}
	if err != nil {
		return ListPage{}, err
	}
	page := ListPage{Items: cards}
	if len(cards) > limit {
		page.Items = cards[:limit]
		last := page.Items[limit-1]
		at := last.UpdatedAt
		if !q.Mine && last.FirstPublishedAt != nil {
			at = *last.FirstPublishedAt
		}
		if page.NextCursor, err = encodeCursor(kind, ports.PageCursor{T: at, ID: last.ID}); err != nil {
			return ListPage{}, err
		}
	}
	return page, nil
}

func parseSearch(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if !utf8.ValidString(s) || strings.ContainsFunc(s, unicode.IsControl) {
		return "", contract.Field("q", contract.ReasonInvalid)
	}
	if utf8.RuneCountInString(s) > MaxSearchLen {
		return "", contract.Field("q", contract.ReasonTooLong)
	}
	return s, nil
}

// parseTypeCode only bounds the value: an unknown type simply matches nothing.
func parseTypeCode(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if len(s) > maxTypeCodeLen || !utf8.ValidString(s) || strings.ContainsFunc(s, unicode.IsControl) {
		return "", contract.Field("type", contract.ReasonInvalid)
	}
	return s, nil
}

// cursorJSON binds the position to the list it came from: entity, scope and filter status.
type cursorJSON struct {
	K  string    `json:"k"`
	T  time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}

func encodeCursor(kind string, c ports.PageCursor) (string, error) {
	if c.ID == uuid.Nil || c.T.IsZero() {
		return "", contract.Field("cursor", contract.ReasonInvalid)
	}
	raw, err := json.Marshal(cursorJSON{K: kind, T: c.T.UTC(), ID: c.ID})
	if err != nil {
		return "", contract.Field("cursor", contract.ReasonInvalid)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeCursor(raw, kind string) (*ports.PageCursor, error) {
	if raw == "" {
		return nil, nil
	}
	invalid := contract.Field("cursor", contract.ReasonInvalid)
	if len(raw) > maxCursorLen {
		return nil, invalid
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalid
	}
	var c cursorJSON
	if err := json.Unmarshal(data, &c); err != nil || c.K != kind || c.ID == uuid.Nil || c.T.IsZero() {
		return nil, invalid
	}
	return &ports.PageCursor{T: c.T.UTC(), ID: c.ID}, nil
}
