package write

import "testing"

func TestVoteLikesDelta(t *testing.T) {
	like := true
	dislike := false
	cases := []struct {
		name   string
		prev   *bool
		isLike bool
		want   int
	}{
		{name: "new like", prev: nil, isLike: true, want: 1},
		{name: "new dislike", prev: nil, isLike: false, want: 0},
		{name: "same like", prev: &like, isLike: true, want: 0},
		{name: "like to dislike", prev: &like, isLike: false, want: -1},
		{name: "dislike to like", prev: &dislike, isLike: true, want: 1},
	}
	for _, tc := range cases {
		if got := voteLikesDelta(tc.prev, tc.isLike); got != tc.want {
			t.Fatalf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}
