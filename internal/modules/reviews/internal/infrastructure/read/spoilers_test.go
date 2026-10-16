package read

import "testing"

func TestHasSpoilerMarkup(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"привет это не спойлер", false},
		{"привет это не спойлер,|| а вот это уже спойлер||", true},
		{"||spoiler||", true},
		{"только открытие ||", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := hasSpoilerMarkup(tc.body); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.body, got, tc.want)
		}
	}
}
