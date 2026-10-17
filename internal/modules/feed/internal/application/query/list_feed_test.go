package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/feed/contract"
	"review-api/internal/modules/feed/internal/application/ports"
)

type fakeReader struct {
	last  ports.ListParams
	items []ports.Item
}

func (f *fakeReader) List(_ context.Context, p ports.ListParams) ([]ports.Item, error) {
	f.last = p
	return f.items, nil
}

func TestEncodeDecodeCursor_roundTrip(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ts := time.Date(2026, 10, 17, 12, 0, 0, 123456789, time.UTC)
	raw, err := EncodeCursor(ports.Cursor{Sort: ports.SortLikes, V: 42, T: ts, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCursor(raw, ports.SortLikes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sort != ports.SortLikes || got.V != 42 || !got.T.Equal(ts) || got.ID != id {
		t.Fatalf("got %+v", got)
	}
}

func TestDecodeCursor_rejectsSortMismatch(t *testing.T) {
	id := uuid.New()
	raw, err := EncodeCursor(ports.Cursor{Sort: ports.SortNew, V: 0, T: time.Now().UTC(), ID: id})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeCursor(raw, ports.SortViews)
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "cursor" {
		t.Fatalf("want cursor field error, got %v", err)
	}
}

func TestListFeedHandler_defaultsAndNextCursor(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now().UTC()
	repo := &fakeReader{items: []ports.Item{
		{ID: id1, CreatedAt: now, LikesCount: 3, ViewsCount: 10},
		{ID: id2, CreatedAt: now.Add(-time.Hour), LikesCount: 1, ViewsCount: 5},
		{ID: uuid.New(), CreatedAt: now.Add(-2 * time.Hour)},
	}}
	h := NewListFeedHandler(repo)

	page, err := h.Handle(context.Background(), ListInput{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if repo.last.Sort != ports.SortNew || repo.last.Mode != ports.ModeAll || repo.last.Limit != 3 {
		t.Fatalf("params %+v", repo.last)
	}
	if len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("page items=%d next=%q", len(page.Items), page.NextCursor)
	}
	cur, err := DecodeCursor(page.NextCursor, ports.SortNew)
	if err != nil || cur.ID != id2 {
		t.Fatalf("next cursor %+v err=%v", cur, err)
	}
}

func TestListFeedHandler_rejectsBadSortAndFollowingWithoutViewer(t *testing.T) {
	h := NewListFeedHandler(&fakeReader{})
	_, err := h.Handle(context.Background(), ListInput{Sort: "hot"})
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "sort" {
		t.Fatalf("want sort error, got %v", err)
	}
	_, err = h.Handle(context.Background(), ListInput{Mode: ports.ModeFollowing})
	if !errors.Is(err, contract.ErrUnauthorized) {
		t.Fatalf("want unauthorized, got %v", err)
	}
}
