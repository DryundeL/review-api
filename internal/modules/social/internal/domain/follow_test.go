package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewFollow_rejectsSelf(t *testing.T) {
	id := uuid.New()
	if _, err := NewFollow(id, id); err == nil {
		t.Fatal("expected self-follow rejected")
	}
	a, b := uuid.New(), uuid.New()
	f, err := NewFollow(a, b)
	if err != nil || f.FollowerID != a || f.FolloweeID != b {
		t.Fatalf("got %+v %v", f, err)
	}
}
