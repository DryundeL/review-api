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

	// Kind filters what the feed mixes.
	KindAll     = "all"
	KindReviews = "reviews"
	KindLists   = "lists"

	// ItemReview and ItemList tell feed items apart.
	ItemReview = "review"
	ItemList   = "list"
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

// ListRef is what a list result card shows; type, format and template title come from its snapshot.
type ListRef struct {
	TypeCode      string
	Format        string
	CoverURL      string
	TemplateTitle string
}

// Item is one feed entry: a published review (list shape, no blocks) or a public list result.
// Reviews fill MediaObject, Rating and HasSpoilers; lists fill List.
// ViewerIsLike: true=like, false=dislike, nil=no vote / anonymous viewer.
// CreatedAt is the sort time: creation of a review, first publication of a list result.
type Item struct {
	Kind          string
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
	ViewerIsLike  *bool
	CreatedAt     time.Time
	List          ListRef
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
	Kind     string
	Type     string
	ViewerID uuid.UUID // required when ModeFollowing
	Cursor   *Cursor
	Limit    int
}

type Reader interface {
	List(ctx context.Context, p ListParams) ([]Item, error)
}
