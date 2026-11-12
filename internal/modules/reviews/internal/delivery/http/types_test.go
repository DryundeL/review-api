package reviewshttp

import (
	"encoding/json"
	"testing"

	"review-api/internal/modules/reviews/internal/application/ports"
)

func TestTypeResponsesIncludeImageURL(t *testing.T) {
	const url = "/storage/review-types/manga.webp"
	item := ports.Type{Code: "manga", Label: "Манга", ImageURL: url, IsActive: true}
	for name, response := range map[string]typeResponse{
		"public": publicTypes([]ports.Type{item})[0],
		"admin":  adminType(item),
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
			if body["imageUrl"] != url {
				t.Fatalf("imageUrl = %v, want %s", body["imageUrl"], url)
			}
		})
	}
}
