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

// Review is a read DTO for one review with counters.
type Review struct {
	ID            uuid.UUID
	AuthorID      uuid.UUID
	MediaObjectID uuid.UUID
	Title         string
	Rating        int16
	Status        string
	Blocks        []Block
	LikesCount    int64
	DislikesCount int64
	ViewsCount    int64
}

type ReviewWriter interface {
	Create(ctx context.Context, item domain.Review) error
	Replace(ctx context.Context, item domain.Review) error
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
