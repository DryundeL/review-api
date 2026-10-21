package query

import "testing"

func TestProgressPercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		current, target, want int
	}{
		{0, 10, 0},
		{5, 10, 50},
		{10, 10, 100},
		{15, 10, 100},
		{1, 0, 0},
	}
	for _, tc := range cases {
		if got := ProgressPercent(tc.current, tc.target); got != tc.want {
			t.Fatalf("ProgressPercent(%d,%d)=%d want %d", tc.current, tc.target, got, tc.want)
		}
	}
}
