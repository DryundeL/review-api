package ports

import (
	"context"
	"time"

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

// AuthorRef is the review author (TG nick may be empty).
type AuthorRef struct {
	ID          uuid.UUID
	Username    string
	PhotoURL    string
	IsFollowing bool
	AvatarFrame string
	Backdrop    string
}

// Review is a read DTO for one review with counters.
// Lists omit Blocks and set HasSpoilers; Get fills Blocks.
// ViewerIsLike: true=like, false=dislike, nil=no vote / anonymous viewer.
type Review struct {
	ID            uuid.UUID
	AuthorID      uuid.UUID
	Author        AuthorRef
	MediaObject   MediaObjectRef
	Title         string
	Rating        int16
	Status        string
	Blocks        []Block
	HasSpoilers   bool
	LikesCount    int64
	DislikesCount int64
	ViewsCount    int64
	ViewerIsLike  *bool
}

// Reason is one active report_reasons row for the public catalog.
type Reason struct {
	Code      string
	Label     string
	SortOrder int
}

// Report is a persisted review complaint.
type Report struct {
	ID         uuid.UUID
	ReviewID   uuid.UUID
	ReporterID uuid.UUID
	ReasonCode string
	CreatedAt  time.Time
}

type ReasonReader interface {
	ListActive(ctx context.Context) ([]Reason, error)
	IsActive(ctx context.Context, code string) (bool, error)
}

type ReviewWriter interface {
	Create(ctx context.Context, item domain.Review) error
	Replace(ctx context.Context, item domain.Review) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (domain.Review, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (domain.Review, error)
	MediaType(ctx context.Context, mediaObjectID uuid.UUID) (string, error)
	UpsertVote(ctx context.Context, reviewID, userID uuid.UUID, isLike bool) error
	InsertView(ctx context.Context, reviewID, userID uuid.UUID) (inserted bool, err error)
	InsertReport(ctx context.Context, report Report) error
	CountDistinctReporters(ctx context.Context, reviewID uuid.UUID) (int, error)
	Unpublish(ctx context.Context, reviewID uuid.UUID) error
}

type ReviewReader interface {
	TopByMedia(ctx context.Context, mediaObjectID, viewerID uuid.UUID) ([]Review, error)
	Get(ctx context.Context, id, viewerID uuid.UUID) (Review, error)
	ListByMedia(ctx context.Context, mediaObjectID, viewerID uuid.UUID, limit int) ([]Review, error)
	ListByAuthor(ctx context.Context, authorID, viewerID uuid.UUID, publishedOnly bool, limit int) ([]Review, error)
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
