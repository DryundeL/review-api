package reviewshttp

import (
	"github.com/google/uuid"
	"review-api/internal/modules/reviews/internal/application/ports"
	"testing"
)

func TestReviewAuthorCosmeticsAreURLs(t *testing.T) {
	got := authorResponse(ports.AuthorRef{ID: uuid.New(), AvatarFrame: "frame_gold", Backdrop: "backdrop_stars"})
	if got.AvatarFrame != "/storage/gamification/frame_gold.webp" || got.Backdrop != "/storage/gamification/backdrop_stars.webp" {
		t.Fatalf("author cosmetics = %+v", got)
	}
	empty := authorResponse(ports.AuthorRef{ID: uuid.New()})
	if empty.AvatarFrame != "" || empty.Backdrop != "" {
		t.Fatalf("unset cosmetics = %+v", empty)
	}
	if authorResponse(ports.AuthorRef{}) != nil {
		t.Fatal("missing author must remain nil")
	}
}
