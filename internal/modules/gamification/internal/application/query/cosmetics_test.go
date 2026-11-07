package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type cosmeticsReader struct{ profile ports.Profile }

func (r cosmeticsReader) GetMe(context.Context, uuid.UUID) (ports.Me, error) { return ports.Me{}, nil }
func (r cosmeticsReader) GetCosmetics(context.Context, uuid.UUID) (ports.Profile, error) {
	return r.profile, nil
}

func TestCosmeticsReturnsImageURLs(t *testing.T) {
	cases := []struct {
		name    string
		profile ports.Profile
		want    contract.Cosmetics
	}{
		{"equipped", ports.Profile{EquippedBackground: "background_blue", EquippedAvatarFrame: "frame_gold", EquippedBackdrop: "backdrop_stars"}, contract.Cosmetics{Background: "/storage/gamification/background_blue.webp", AvatarFrame: "/storage/gamification/frame_gold.webp", Backdrop: "/storage/gamification/backdrop_stars.webp"}},
		{"defaults", ports.Profile{}, contract.Cosmetics{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewCosmeticsHandler(cosmeticsReader{tc.profile}).Handle(context.Background(), uuid.New())
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("cosmetics = %+v, want %+v", got, tc.want)
			}
		})
	}
}
