package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
)

func rating(v int16) *int16 { return &v }

func fieldErr(t *testing.T, err error, field, reason string) {
	t.Helper()
	var fe contract.FieldError
	if !errors.As(err, &fe) || fe.Field != field || fe.Reason != reason {
		t.Fatalf("want %s/%s, got %v", field, reason, err)
	}
}

func TestBodyHTML(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{"<p>Сюжет <strong>отличный</strong><br>🙂 &amp; темп</p>", true},
		{"<h3>Плюсы</h3><ul><li>Плюс</li></ul><blockquote>цитата</blockquote>", true},
		{"<p>||спойлер||</p>", true},
		{"<script>alert(1)</script>", false},
		{"<img src=x onerror=alert(1)>", false},
		{"<span onclick=alert(1)>текст</span>", false},
		{"<a href=javascript:alert(1)>ссылка</a>", false},
		{"<svg onload=alert(1)></svg>", false},
		{"<p style=background:url(x)>текст</p>", false},
		{"<!-- x --><p>a</p>", false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			body, _, err := parseBody(tc.body)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && body != tc.body {
				t.Fatalf("body=%q", body)
			}
		})
	}
}

func TestBodyTextLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		valid bool
	}{
		{"cyrillic 20000", strings.Repeat("я", 20000), true},
		{"cyrillic 20001", strings.Repeat("я", 20001), false},
		{"emoji 20000", strings.Repeat("🙂", 20000), true},
		{"emoji 20001", strings.Repeat("🙂", 20001), false},
		{"entities 20000", strings.Repeat("&amp;", 20000), true},
		{"entities 20001", strings.Repeat("&#128578;", 20001), false},
		{"tags not counted", strings.Repeat("<p>я</p>", 20000), true},
		{"paragraph boundaries not counted", "<p>" + strings.Repeat("я", 10000) + "</p><p>" + strings.Repeat("я", 10000) + "</p>", true},
		{"escaped tag is text", strings.Repeat("&lt;b&gt;", 6667), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseBody(tc.body)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			fieldErr(t, err, "body", ReasonTooLong)
		})
	}
}

func TestNewReviewAllowsEmptyDraft(t *testing.T) {
	item, err := NewReview(uuid.New(), uuid.New(), Draft{})
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != StatusDraft || item.Rating != nil || item.Title != "" || item.Revision != 1 {
		t.Fatalf("unexpected: %+v", item)
	}
	if _, err := NewReview(uuid.New(), uuid.Nil, Draft{}); err == nil {
		t.Fatal("media required")
	}
}

func TestDraftValidation(t *testing.T) {
	item, _ := NewReview(uuid.New(), uuid.New(), Draft{})
	fieldErr(t, item.SaveDraft(Draft{Title: "ok <b>no</b>"}), "title", ReasonInvalid)
	fieldErr(t, item.SaveDraft(Draft{Title: strings.Repeat("я", 257)}), "title", ReasonTooLong)
	fieldErr(t, item.SaveDraft(Draft{Rating: rating(0)}), "rating", ReasonInvalid)
	fieldErr(t, item.SaveDraft(Draft{Rating: rating(11)}), "rating", ReasonInvalid)
	if item.Revision != 1 {
		t.Fatalf("rejected save bumped revision: %d", item.Revision)
	}
	if err := item.SaveDraft(Draft{Title: strings.Repeat("я", 256), Rating: rating(1)}); err != nil {
		t.Fatal(err)
	}
	if item.Revision != 2 {
		t.Fatalf("revision=%d", item.Revision)
	}
}

func TestPublish(t *testing.T) {
	newItem := func(d Draft) Review {
		item, err := NewReview(uuid.New(), uuid.New(), d)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	item := newItem(Draft{Body: "<p>текст</p>"})
	_, err := item.Publish()
	fieldErr(t, err, "rating", ReasonRequired)

	for _, empty := range []string{"", "   ", "<p></p>", "<p> </p><br>", "<p>&nbsp;</p>"} {
		item = newItem(Draft{Rating: rating(5), Body: empty})
		_, err = item.Publish()
		fieldErr(t, err, "body", ReasonRequired)
		if item.Status != StatusDraft || item.PublishedRevision != nil {
			t.Fatalf("failed publish changed state: %+v", item)
		}
	}

	for _, r := range []int16{1, 10} {
		item = newItem(Draft{Rating: rating(r), Body: "<p>текст</p>"})
		changed, err := item.Publish()
		if err != nil || !changed || item.Status != StatusPublished || *item.PublishedRevision != item.Revision {
			t.Fatalf("publish without title: %v %+v", err, item)
		}
	}

	changed, err := item.Publish()
	if err != nil || changed {
		t.Fatalf("repeat publish must be a no-op: %v %v", changed, err)
	}

	if err := item.SaveDraft(Draft{Body: ""}); err != nil {
		t.Fatal(err)
	}
	if item.Status != StatusPublished || *item.PublishedRevision != item.Revision-1 {
		t.Fatalf("draft save must not touch publication: %+v", item)
	}
	if _, err := item.Publish(); err == nil {
		t.Fatal("incomplete edits must not be published")
	}
}
