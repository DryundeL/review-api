package domain

import (
	"errors"
	"strings"
	"testing"

	"review-api/internal/modules/reviews/contract"
)

func TestNewReviewTypeAcceptsClientCode(t *testing.T) {
	t.Parallel()
	item, err := NewReviewType("light_novel", "Ранобэ", 40, true)
	if err != nil {
		t.Fatal(err)
	}
	if item.Code != "light_novel" || item.Label != "Ранобэ" || !item.IsActive {
		t.Fatalf("item = %+v", item)
	}
}

func TestNewReviewTypeRejectsUnsafeValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		code  string
		label string
		sort  int
		field string
	}{
		{name: "space", code: "light novel", label: "Ранобэ", sort: 1, field: "code"},
		{name: "upper", code: "Manga", label: "Манга", sort: 1, field: "code"},
		{name: "label", code: "manga", label: " ", sort: 1, field: "label"},
		{name: "long", code: "manga", label: strings.Repeat("я", 65), sort: 1, field: "label"},
		{name: "sort", code: "manga", label: "Манга", sort: -1, field: "sortOrder"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewReviewType(tc.code, tc.label, tc.sort, true)
			var field contract.FieldError
			if !errors.As(err, &field) || field.Field != tc.field {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
