package ports

import (
	"context"

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
