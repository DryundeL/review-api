package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

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
