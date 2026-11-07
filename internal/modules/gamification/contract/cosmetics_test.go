package contract

import "testing"

func TestCosmeticURL(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{"bg_ivory_bloom", "/storage/gamification/bg_ivory_bloom.webp"},
		{"frame_ivory", "/storage/gamification/frame_ivory.webp"},
		{"list_bloom", "/storage/gamification/list_bloom.webp"},
		{"", ""},
		{"../secret", ""},
		{"https://example.com/image.webp", ""},
	} {
		if got := CosmeticURL(tc.code); got != tc.want {
			t.Errorf("CosmeticURL(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
