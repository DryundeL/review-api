package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/catalog/contract"
)

func TestNewUserWork(t *testing.T) {
	t.Parallel()
	year := 1999
	work, err := NewUserWork(uuid.MustParse("11111111-1111-1111-1111-111111111111"), Draft{
		Title:         " Матрица\n",
		PrimaryType:   "movie",
		Types:         []string{"movie", "anime_movie", "movie"},
		ReleaseYear:   &year,
		CoverURL:      "https://cdn.example/cover.jpg",
		ReleaseStatus: "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if work.Title != "Матрица" || work.Origin != "user" || work.PrimaryType != "movie" {
		t.Fatalf("work = %+v", work)
	}
	if len(work.Types) != 2 || work.Types[0] != "movie" || work.Types[1] != "anime_movie" {
		t.Fatalf("types = %v", work.Types)
	}
}

func TestNewUserWorkRejectsCoverAndStatus(t *testing.T) {
	t.Parallel()
	author := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cases := []struct {
		name  string
		draft Draft
		field string
	}{
		{name: "status", draft: Draft{Title: "Фильм", PrimaryType: "movie", ReleaseStatus: "banned"}, field: "releaseStatus"},
		{name: "cover", draft: Draft{Title: "Фильм", PrimaryType: "movie", ReleaseStatus: "completed", CoverURL: "http://cdn.example/a.jpg"}, field: "coverUrl"},
		{name: "private", draft: Draft{Title: "Фильм", PrimaryType: "movie", ReleaseStatus: "completed", CoverURL: "https://127.0.0.1/a.jpg"}, field: "coverUrl"},
		{name: "year", draft: Draft{Title: "Фильм", PrimaryType: "movie", ReleaseStatus: "completed", ReleaseYear: intPtr(1700)}, field: "releaseYear"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewUserWork(author, tc.draft)
			var field contract.FieldError
			if !errors.As(err, &field) || field.Field != tc.field {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func intPtr(v int) *int { return &v }
