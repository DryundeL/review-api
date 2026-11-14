package query

import (
	"github.com/google/uuid"

	"review-api/internal/modules/reviews/internal/application/ports"
	"strings"
	"testing"
	"time"
)

func TestCommentCursorBoundToBranch(t *testing.T) {
	review, parent := uuid.New(), uuid.New()
	pos := ports.CommentPosition{ID: uuid.New(), CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 123000, time.UTC)}
	cursor := encodeCommentCursor(review, &parent, pos)
	got, err := decodeCommentCursor(cursor, review, &parent)
	if err != nil || got.ID != pos.ID || !got.CreatedAt.Equal(pos.CreatedAt) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	for _, tc := range []struct {
		cursor string
		review uuid.UUID
		parent *uuid.UUID
	}{
		{cursor, uuid.New(), &parent}, {cursor, review, nil},
		{"bad!", review, &parent}, {strings.Repeat("a", 1025), review, &parent},
	} {
		if _, err := decodeCommentCursor(tc.cursor, tc.review, tc.parent); err == nil {
			t.Fatal("accepted invalid cursor")
		}
	}
}
