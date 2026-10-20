package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	SortNew   = "new"
	SortViews = "views"
	SortLikes = "likes"

	ModeAll       = "all"
	ModeFollowing = "following"
)

// MediaObjectRef is a compact catalog title embedded in feed items.
type MediaObjectRef struct {
	ID        uuid.UUID
	Title     string
	Type      string
	PosterURL string
}

// AuthorRef is the review author for feed cards (TG nick may be empty).
type AuthorRef struct {
	ID          uuid.UUID
	Username    string
	PhotoURL    string
	IsFollowing bool
	AvatarFrame string
	Backdrop    string
}

// Item is one published review in the feed (list shape, no blocks).
type Item struct {
	ID            uuid.UUID
	AuthorID      uuid.UUID
	Author        AuthorRef
	MediaObject   MediaObjectRef
	Title         string
	Rating        int16
	HasSpoilers   bool
	LikesCount    int64
	DislikesCount int64
	ViewsCount    int64
	CreatedAt     time.Time
}

// Cursor is a keyset position for the active sort.
type Cursor struct {
	Sort string
	V    int64
	T    time.Time
	ID   uuid.UUID
}

// ListParams drives a single feed page.
type ListParams struct {
	Sort     string
	Mode     string
	Type     string
	ViewerID uuid.UUID // required when ModeFollowing
	Cursor   *Cursor
	Limit    int
}

type Reader interface {
	List(ctx context.Context, p ListParams) ([]Item, error)
}
