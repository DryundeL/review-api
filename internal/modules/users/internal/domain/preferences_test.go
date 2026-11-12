package domain

import "testing"

func TestContentTypes(t *testing.T) {
	for _, in := range [][]string{nil, {}, {"manga", "manga"}, {"Manga"}, {"../manga"}, {""}} {
		if _, err := ContentTypes(in); err == nil {
			t.Fatalf("accepted %v", in)
		}
	}
	got, err := ContentTypes([]string{"manga", "book"})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}
}
