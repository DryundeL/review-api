package gamificationhttp

import (
	"encoding/json"
	"testing"

	"review-api/internal/modules/gamification/internal/application/ports"
)

func TestCatalogResponsesIncludeAssetURL(t *testing.T) {
	shop := ports.ShopItem{Code: "frame_gold", URL: "/storage/gamification/frame_gold.webp"}
	achievement := ports.AchievementItem{Code: "first_review", URL: "/storage/achievements/first_review.webp"}
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

func TestEquippedResponsesReturnImageURLs(t *testing.T) {
	profile := ports.Profile{EquippedBackground: "background_blue", EquippedAvatarFrame: "frame_gold", EquippedBackdrop: "backdrop_stars"}
	want := equippedResponse{Background: "/storage/gamification/background_blue.webp", AvatarFrame: "/storage/gamification/frame_gold.webp", Backdrop: "/storage/gamification/backdrop_stars.webp"}
	if got := equippedJSON(profile); got != want {
		t.Fatalf("equipped = %+v, want %+v", got, want)
	}
	if got := meJSON(ports.Me{Profile: profile}).Equipped; got != want {
		t.Fatalf("me equipped = %+v, want %+v", got, want)
	}
	data, err := json.Marshal(equippedJSON(ports.Profile{}))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("empty equipped = %s, want {}", data)
	}
}
