package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBlockBodyHTML(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{"<p>Сюжет <strong>отличный</strong><br>🙂 &amp; темп</p>", true},
		{"<ul><li>Плюс</li></ul>", true},
		{"<script>alert(1)</script>", false},
		{"<img src=x onerror=alert(1)>", false},
		{"<span onclick=alert(1)>текст</span>", false},
		{"<a href=javascript:alert(1)>ссылка</a>", false},
		{"<svg onload=alert(1)></svg>", false},
		{"<p style=background:url(x)>текст</p>", false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			body, err := parseBlockBody(tc.body)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && body != tc.body {
				t.Fatalf("body=%q", body)
			}
		})
	}
}

func TestReviewBlockBodyHasNoLengthLimit(t *testing.T) {
	for _, length := range []int{500, 501} {
		draft := Draft{MediaObjectID: uuid.New(), Title: "Обзор", Rating: 5,
			Blocks: []BlockDraft{{Title: "Блок", Body: strings.Repeat("я", length)}}}
		_, err := NewReview(uuid.New(), draft)
		if err != nil {
			t.Fatalf("create length=%d: %v", length, err)
		}
		item := Review{Status: StatusDraft}
		err = item.Apply(draft)
		if err != nil {
			t.Fatalf("update length=%d: %v", length, err)
		}
	}
}

func TestNewReview_rejectsMarkupAndEmpty(t *testing.T) {
	author := uuid.New()
	media := uuid.New()

	_, err := NewReview(author, Draft{
		MediaObjectID: media,
		Title:         "ok <b>no</b>",
		Rating:        5,
		Blocks:        []BlockDraft{{Title: "t", Body: "b"}},
	})
	if err == nil {
		t.Fatal("expected markup title rejected")
	}

	_, err = NewReview(author, Draft{
		MediaObjectID: media,
		Title:         "Норм обзор 🙂",
		Rating:        11,
		Blocks:        nil,
	})
	if err == nil {
		t.Fatal("expected rating rejected")
	}

	item, err := NewReview(author, Draft{
		MediaObjectID: media,
		Title:         "Норм обзор 🙂",
		Rating:        8,
		Status:        StatusPublished,
		Blocks:        []BlockDraft{{Title: "Плюсы", Body: "Сюжет &amp; темп"}},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if item.Status != StatusPublished || len(item.Blocks) != 1 || item.Blocks[0].Position != 1 {
		t.Fatalf("unexpected item: %+v", item)
	}
}
