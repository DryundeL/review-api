package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewCommentValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"trim", "  Привет 🙂  ", true},
		{"unicode boundary", strings.Repeat("🙂", 2000), true},
		{"too long", strings.Repeat("я", 2001), false},
		{"empty", " \n\t", false},
		{"invalid utf8", string([]byte{0xff}), false},
		{"nul", "a\x00b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewComment(uuid.New(), uuid.New(), nil, tc.body)
			if (err == nil) != tc.valid {
				t.Fatalf("comment=%+v err=%v", c, err)
			}
			if tc.valid && (c.Body != strings.TrimSpace(tc.body) || c.ID == uuid.Nil || c.CreatedAt.IsZero()) {
				t.Fatalf("comment=%+v", c)
			}
		})
	}
	id := uuid.New()
	zero := uuid.Nil
	for _, args := range []struct {
		review, author uuid.UUID
		parent         *uuid.UUID
	}{
		{uuid.Nil, id, nil}, {id, uuid.Nil, nil}, {id, id, &zero},
	} {
		if _, err := NewComment(args.review, args.author, args.parent, "text"); err == nil {
			t.Fatal("accepted nil ID")
		}
	}
}
