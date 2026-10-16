package domain

import (
	"github.com/google/uuid"

	"review-api/internal/modules/social/contract"
)

// Follow is a directed subscription edge.
type Follow struct {
	FollowerID uuid.UUID
	FolloweeID uuid.UUID
}

func NewFollow(followerID, followeeID uuid.UUID) (Follow, error) {
	if followerID == uuid.Nil {
		return Follow{}, contract.FieldError{Field: "followerId"}
	}
	if followeeID == uuid.Nil {
		return Follow{}, contract.FieldError{Field: "followeeId"}
	}
	if followerID == followeeID {
		return Follow{}, contract.FieldError{Field: "followeeId"}
	}
	return Follow{FollowerID: followerID, FolloweeID: followeeID}, nil
}
