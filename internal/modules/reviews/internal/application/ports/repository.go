package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/internal/domain"
)

// Type is a review_types row returned to the API.
type Type struct {
	Code      string
	Label     string
	SortOrder int
	IsActive  bool
}

type Reader interface {
	List(ctx context.Context, activeOnly bool) ([]Type, error)
}

type Writer interface {
	Create(ctx context.Context, item domain.ReviewType) error
	Update(ctx context.Context, code string, label *string, sortOrder *int, active *bool) (domain.ReviewType, error)
	NextSort(ctx context.Context) (int, error)
}

// Block is a read DTO for one review section.
type Block struct {
	ID       uuid.UUID
	Position int16
	Title    string
	Body     string
}

// MediaObjectRef is a compact catalog title embedded in review responses.
type MediaObjectRef struct {
	ID        uuid.UUID
	Title     string
	Type      string
	PosterURL string
}

// Review is a read DTO for one review with counters.
// Lists omit Blocks and set HasSpoilers; Get fills Blocks.
type Review struct {
	ID            uuid.UUID
	AuthorID      uuid.UUID
	MediaObject   MediaObjectRef
	Title         string
	Rating        int16
	Status        string
	Blocks        []Block
	HasSpoilers   bool
	LikesCount    int64
	DislikesCount int64
	ViewsCount    int64
}

type ReviewWriter interface {
	Create(ctx context.Context, item domain.Review) error
	Replace(ctx context.Context, item domain.Review) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (domain.Review, error)
	UpsertVote(ctx context.Context, reviewID, userID uuid.UUID, isLike bool) error
	InsertView(ctx context.Context, reviewID, userID uuid.UUID) (inserted bool, err error)
}

type ReviewReader interface {
	Get(ctx context.Context, id uuid.UUID) (Review, error)
	ListByMedia(ctx context.Context, mediaObjectID uuid.UUID, limit int) ([]Review, error)
	ListByAuthor(ctx context.Context, authorID uuid.UUID, publishedOnly bool, limit int) ([]Review, error)
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
