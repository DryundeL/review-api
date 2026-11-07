package contract

import "testing"

func TestCosmeticURL(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{"frame_gold", "/storage/gemification/frame_gold.webp"},
		{"", ""},
		{"../secret", ""},
		{"https://example.com/image.webp", ""},
	} {
		if got := CosmeticURL(tc.code); got != tc.want {
			t.Errorf("CosmeticURL(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
