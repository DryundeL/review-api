package reviewshttp

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/http/httperr"
)

func TestParseReviewDraftBlockBodyLength(t *testing.T) {
	for _, requireMedia := range []bool{true, false} {
		for _, tc := range []struct {
			text  string
			valid bool
		}{
			{strings.Repeat("я", 500), true},
			{strings.Repeat("я", 501), false},
			{strings.Repeat(" ", 501), false},
			{"<p><strong>" + strings.Repeat("я", 500) + "</strong></p>", true},
			{"<p>" + strings.Repeat("я", 501) + "</p>", false},
			{strings.Repeat("<b>я</b>", 500), true},
			{strings.Repeat("<b>я</b>", 501), false},
			{strings.Repeat("&amp;", 500), true},
			{strings.Repeat("&#128578;", 500), true},
			{strings.Repeat("&amp;", 501), false},
			{"<p>" + strings.Repeat("&lt;b&gt;", 167) + "</p>", false},
		} {
			text := tc.text
			body := reviewBody{MediaObjectID: uuid.NewString(), Blocks: []reviewBlockBody{{Title: "Блок", Body: text}}}
			_, err := parseReviewDraft(body, requireMedia)
			if tc.valid {
				if err != nil {
					t.Fatalf("500 characters rejected: %v", err)
				}
				continue
			}
			var httpErr *httperr.Error
			if err == nil || !errors.As(mapReviewError(err), &httpErr) || httpErr.Status != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422 for oversized block: %v", err)
			}
			details, ok := httpErr.Details.(map[string]string)
			if !ok || details["field"] != "blocks" {
				t.Fatalf("unexpected error details: %v", httpErr.Details)
			}
		}
	}
}
