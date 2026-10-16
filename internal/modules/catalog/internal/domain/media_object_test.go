package domain

import (
	"errors"
	"strings"
	"testing"

	"review-api/internal/modules/catalog/contract"
)

func TestNewImported(t *testing.T) {
	t.Parallel()
	year := 1999
	item, err := NewImported(Draft{
		Title:         " Матрица\n",
		OriginalTitle: " The Matrix ",
		Type:          "movie",
		Genre:         "sci-fi",
		ReleaseYear:   &year,
		PosterURL:     "https://cdn.example/poster.jpg",
		ReleaseStatus: "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Матрица" || item.OriginalTitle != "The Matrix" || item.Genre != "sci-fi" || item.Type != "movie" || item.PosterURL == "" || item.CreatedBy != nil {
		t.Fatalf("item = %+v", item)
	}
}

func TestNewImportedRejectsPosterAndStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		draft Draft
		field string
	}{
		{name: "status", draft: Draft{Title: "Фильм", Type: "movie", ReleaseStatus: "banned"}, field: "releaseStatus"},
		{name: "poster", draft: Draft{Title: "Фильм", Type: "movie", ReleaseStatus: "completed", PosterURL: "http://cdn.example/a.jpg"}, field: "posterUrl"},
		{name: "private", draft: Draft{Title: "Фильм", Type: "movie", ReleaseStatus: "completed", PosterURL: "https://127.0.0.1/a.jpg"}, field: "posterUrl"},
		{name: "year", draft: Draft{Title: "Фильм", Type: "movie", ReleaseStatus: "completed", ReleaseYear: intPtr(1700)}, field: "releaseYear"},
		{name: "external", draft: Draft{Title: "Фильм", Type: "movie", ReleaseStatus: "completed", ExternalSource: "tmdb"}, field: "external"},
		{name: "genre", draft: Draft{Title: "Фильм", Type: "movie", ReleaseStatus: "completed", Genre: strings.Repeat("a", 101)}, field: "genre"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewImported(tc.draft)
			var field contract.FieldError
			if !errors.As(err, &field) || field.Field != tc.field {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func intPtr(v int) *int { return &v }
