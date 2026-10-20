package ports

import (
	"context"

	"github.com/google/uuid"
)

const (
	TabMedia   = "media"
	TabAuthors = "authors"
	TabTypes   = "types"
)

// MediaTrend is a catalog title ranked by published review count.
type MediaTrend struct {
	ID           uuid.UUID
	Title        string
	Type         string
	PosterURL    string
	ReviewsCount int64
}

// AuthorTrend is a user ranked by follower count.
type AuthorTrend struct {
	ID             uuid.UUID
	Username       string
	FirstName      string
	PhotoURL       string
	FollowersCount int64
	IsFollowing    bool
}

// TypeTrend is a review type ranked by published review count.
type TypeTrend struct {
	Code         string
	Label        string
	ReviewsCount int64
}

// UserHit is a username match in text search.
type UserHit struct {
	ID             uuid.UUID
	Username       string
	FirstName      string
	PhotoURL       string
	FollowersCount int64
	IsFollowing    bool
}

// ReviewHit is a published review title match.
type ReviewHit struct {
	ID          uuid.UUID
	Title       string
	AuthorID    uuid.UUID
	Username    string
	PhotoURL    string
	AvatarFrame string
	Backdrop    string
	MediaObject MediaRef
	Rating      int16
	LikesCount  int64
	ViewsCount  int64
}

// MediaHit is a media object title match.
type MediaHit struct {
	ID        uuid.UUID
	Title     string
	Type      string
	PosterURL string
}

// MediaRef is a compact catalog title on a review hit.
type MediaRef struct {
	ID        uuid.UUID
	Title     string
	Type      string
	PosterURL string
}

type Reader interface {
	TopMedia(ctx context.Context, limit int) ([]MediaTrend, error)
	TopAuthors(ctx context.Context, viewerID uuid.UUID, limit int) ([]AuthorTrend, error)
	TopTypes(ctx context.Context) ([]TypeTrend, error)
	SearchUsers(ctx context.Context, q string, viewerID uuid.UUID, limit int) ([]UserHit, error)
	SearchReviews(ctx context.Context, q string, limit int) ([]ReviewHit, error)
	SearchMedia(ctx context.Context, q string, limit int) ([]MediaHit, error)
}
