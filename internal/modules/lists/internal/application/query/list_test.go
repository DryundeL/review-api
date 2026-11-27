package query

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

// fakeLists returns n cards with decreasing times and records the last query.
type fakeLists struct {
	n    int
	last ports.ListQuery
}

func (f *fakeLists) cards(q ports.ListQuery) []ports.ListCard {
	f.last = q
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	out := []ports.ListCard{}
	for i := range min(f.n, q.Limit) {
		at := base.Add(-time.Duration(i) * time.Hour)
		out = append(out, ports.ListCard{ID: uuid.New(), FirstPublishedAt: &at, UpdatedAt: at.Add(time.Minute)})
	}
	return out
}

func (f *fakeLists) ListTemplates(_ context.Context, q ports.ListQuery) ([]ports.ListCard, error) {
	return f.cards(q), nil
}

func (f *fakeLists) ListResults(_ context.Context, q ports.ListQuery) ([]ports.ListCard, error) {
	return f.cards(q), nil
}

func wantField(t *testing.T, err error, field, reason string) {
	t.Helper()
	fe, ok := errors.AsType[contract.FieldError](err)
	if !ok || fe.Field != field || fe.Reason != reason {
		t.Fatalf("err = %v, want %s/%s", err, field, reason)
	}
}

func TestListPaging(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeLists{n: 3}
	h := NewListHandler(repo)

	page, err := h.Templates(ctx, ListInput{Limit: 2, Search: "  Игры ", Format: "ranked", Type: "game"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.NextCursor == "" || repo.last.Limit != 3 || repo.last.Mine {
		t.Fatalf("page = %+v, query = %+v", page, repo.last)
	}
	if repo.last.Search != "Игры" || repo.last.Format != domain.FormatRanked || repo.last.TypeCode != "game" {
		t.Fatalf("filters = %+v", repo.last)
	}
	if _, err := h.Templates(ctx, ListInput{Cursor: page.NextCursor}); err != nil {
		t.Fatal(err)
	}
	if after := repo.last.After; after == nil || !after.T.Equal(*page.Items[1].FirstPublishedAt) || after.ID != page.Items[1].ID {
		t.Fatalf("cursor = %+v", repo.last.After)
	}
	// A cursor is valid only for the list it came from.
	if _, err := h.Results(ctx, ListInput{Cursor: page.NextCursor}); err == nil {
		t.Fatal("templates cursor accepted by results")
	}
	if _, err := h.ProfileTemplates(ctx, uuid.New(), ListInput{Cursor: page.NextCursor}); err == nil {
		t.Fatal("section cursor accepted by a profile")
	}

	// The author's list pages by last update; the status is part of the cursor.
	author := uuid.New()
	mine, err := h.MyResults(ctx, author, MineInput{Status: "draft", Limit: 1})
	if err != nil || !repo.last.Mine || repo.last.AuthorID != author || repo.last.Status != ports.MineDraft {
		t.Fatalf("mine: %v %+v", err, repo.last)
	}
	if _, err := h.MyResults(ctx, author, MineInput{Status: "draft", Cursor: mine.NextCursor}); err != nil {
		t.Fatal(err)
	}
	if !repo.last.After.T.Equal(mine.Items[0].UpdatedAt) {
		t.Fatalf("mine cursor uses %v", repo.last.After.T)
	}
	if _, err := h.MyResults(ctx, author, MineInput{Status: "hidden", Cursor: mine.NextCursor}); err == nil {
		t.Fatal("cursor of another status accepted")
	}

	repo.n = 2
	if page, err = h.Results(ctx, ListInput{Limit: 2}); err != nil || page.NextCursor != "" {
		t.Fatalf("last page: %v %q", err, page.NextCursor)
	}
	if _, err := h.Results(ctx, ListInput{}); err != nil || repo.last.Limit != DefaultPageSize+1 {
		t.Fatalf("default limit: %v %d", err, repo.last.Limit)
	}
}

func TestListInputValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := NewListHandler(&fakeLists{})
	cases := map[string]struct {
		in     ListInput
		field  string
		reason string
	}{
		"long search":    {ListInput{Search: strings.Repeat("я", MaxSearchLen+1)}, "q", contract.ReasonTooLong},
		"control search": {ListInput{Search: "a\x00b"}, "q", contract.ReasonInvalid},
		"bad format":     {ListInput{Format: "pyramid"}, "format", contract.ReasonInvalid},
		"long type":      {ListInput{Type: strings.Repeat("a", 65)}, "type", contract.ReasonInvalid},
		"big limit":      {ListInput{Limit: MaxPageSize + 1}, "limit", contract.ReasonInvalid},
		"negative limit": {ListInput{Limit: -1}, "limit", contract.ReasonInvalid},
		"garbage cursor": {ListInput{Cursor: "!!!"}, "cursor", contract.ReasonInvalid},
		"json cursor":    {ListInput{Cursor: "e30"}, "cursor", contract.ReasonInvalid}, // {}
		"huge cursor":    {ListInput{Cursor: strings.Repeat("A", 300)}, "cursor", contract.ReasonInvalid},
	}
	for name, tc := range cases {
		_, err := h.Templates(ctx, tc.in)
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		wantField(t, err, tc.field, tc.reason)
	}
	if _, err := h.Templates(ctx, ListInput{Search: strings.Repeat("я", MaxSearchLen)}); err != nil {
		t.Fatalf("search of max length: %v", err)
	}
	_, err := h.MyTemplates(ctx, uuid.New(), MineInput{Status: "deleted"})
	wantField(t, err, "status", contract.ReasonInvalid)
	if _, err := h.MyTemplates(ctx, uuid.Nil, MineInput{}); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("guest mine: %v", err)
	}
}
