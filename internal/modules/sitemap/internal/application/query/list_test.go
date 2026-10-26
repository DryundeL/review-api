package query

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestParseRequest(t *testing.T) {
	id := uuid.New()
	for _, entity := range []string{"media-objects", "reviews", "authors"} {
		req, err := ParseRequest(entity, "", "")
		if err != nil || req.Limit != 500 || req.After != uuid.Nil {
			t.Fatalf("default request: %+v, %v", req, err)
		}
		req, err = ParseRequest(entity, "1000", id.String())
		if err != nil || req.Limit != 1000 || req.After != id {
			t.Fatalf("explicit request: %+v, %v", req, err)
		}
	}
	for _, input := range [][3]string{
		{"users", "", ""}, {"reviews", "0", ""}, {"reviews", "-1", ""},
		{"reviews", "1001", ""}, {"reviews", "abc", ""},
		{"reviews", "99999999999999999999999999", ""},
		{"reviews", "", "bad"}, {"reviews", "", uuid.Nil.String()},
	} {
		if _, err := ParseRequest(input[0], input[1], input[2]); err == nil {
			t.Fatalf("accepted invalid request: %v", input)
		}
	}
}

func TestPagination(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, count := range []int{0, 1, 2, 3} {
		h := NewListHandler(&stubReader{ids: ids[:count]})
		page, err := h.Handle(context.Background(), Request{Entity: "reviews", Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		want := min(count, 2)
		if page.Items == nil || len(page.Items) != want {
			t.Fatalf("page: %+v", page)
		}
		if count > 2 {
			if page.NextCursor != ids[1].String() {
				t.Fatalf("cursor: %q", page.NextCursor)
			}
		} else if page.NextCursor != "" {
			t.Fatalf("unexpected cursor: %q", page.NextCursor)
		}
		for i, item := range page.Items {
			if item.ID != ids[i] {
				t.Fatal("ID changed")
			}
		}
	}
	errDB := errors.New("database unavailable")
	if _, err := NewListHandler(&stubReader{err: errDB}).Handle(context.Background(), Request{Entity: "reviews", Limit: 2}); !errors.Is(err, errDB) {
		t.Fatalf("error lost: %v", err)
	}
}

type stubReader struct {
	ids []uuid.UUID
	err error
}

func (s *stubReader) List(context.Context, string, uuid.UUID, int) ([]uuid.UUID, error) {
	return s.ids, s.err
}
