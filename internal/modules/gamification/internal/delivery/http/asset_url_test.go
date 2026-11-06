package gamificationhttp

import (
	"encoding/json"
	"testing"

	"review-api/internal/modules/gamification/internal/application/ports"
)

func TestCatalogResponsesIncludeAssetURL(t *testing.T) {
	shop := ports.ShopItem{Code: "frame_gold", URL: "/storage/gemification/frame_gold.webp"}
	achievement := ports.AchievementItem{Code: "first_review", URL: "/storage/achevments/first_review.webp"}
	cases := []struct {
		name     string
		response any
		want     string
	}{
		{"public shop", publicItems([]ports.ShopItem{shop})[0], shop.URL},
		{"admin shop", adminItem(shop), shop.URL},
		{"public achievement", progressItems([]ports.AchievementProgress{{AchievementItem: achievement}})[0], achievement.URL},
		{"admin achievement", adminAchievementItem(achievement), achievement.URL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.response)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got["url"] != tc.want {
				t.Fatalf("url = %v, want %s", got["url"], tc.want)
			}
		})
	}
}
