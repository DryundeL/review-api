package query

import "testing"

func TestNormalizeQuery(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"@ann", "ann"},
		{" @ann ", "ann"},
		{"manga", "manga"},
		{"@@x", "@x"},
	}
	for _, tc := range cases {
		if got := NormalizeQuery(tc.in); got != tc.want {
			t.Fatalf("NormalizeQuery(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
