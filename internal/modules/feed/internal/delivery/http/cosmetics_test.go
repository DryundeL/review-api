package feedhttp

import (
	"review-api/internal/modules/feed/internal/application/ports"
	"testing"
)

func TestFeedAuthorCosmeticsAreURLs(t *testing.T) {
	item := ports.Item{}
	item.Author.AvatarFrame = "frame_gold"
	item.Author.Backdrop = "backdrop_stars"
	got := itemJSON(item).Author
	if got.AvatarFrame != "/storage/gemification/frame_gold.webp" || got.Backdrop != "/storage/gemification/backdrop_stars.webp" {
		t.Fatalf("author cosmetics = %+v", got)
	}
	empty := itemJSON(ports.Item{}).Author
	if empty.AvatarFrame != "" || empty.Backdrop != "" {
		t.Fatalf("unset cosmetics = %+v", empty)
	}
}
