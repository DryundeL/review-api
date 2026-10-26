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
		for _, text := range []string{strings.Repeat("я", 500), strings.Repeat("я", 501), strings.Repeat(" ", 501)} {
			body := reviewBody{MediaObjectID: uuid.NewString(), Blocks: []reviewBlockBody{{Title: "Блок", Body: text}}}
			_, err := parseReviewDraft(body, requireMedia)
			if text == strings.Repeat("я", 500) {
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
