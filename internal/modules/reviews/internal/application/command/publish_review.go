package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

type PublishReviewHandler struct {
	repo      ports.ReviewWriter
	reader    ports.ReviewReader
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewPublishReviewHandler(
	repo ports.ReviewWriter,
	reader ports.ReviewReader,
	tx transaction.Manager,
	publisher ports.Publisher,
) *PublishReviewHandler {
	return &PublishReviewHandler{repo: repo, reader: reader, tx: tx, publisher: publisher}
}

// Handle publishes exactly the given saved revision ("Опубликовать" and "Обновить публикацию").
// Repeating it for the already public revision is a no-op; updating a public review emits nothing.
func (h *PublishReviewHandler) Handle(ctx context.Context, reviewID, authorID uuid.UUID, revision int64) (ports.EditorReview, error) {
	if h == nil || h.repo == nil || h.reader == nil || h.tx == nil || h.publisher == nil {
		return ports.EditorReview{}, contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		item, err := lockOwnReview(txCtx, h.repo, reviewID, authorID, revision)
		if err != nil {
			return err
		}
		wasPublished := item.Status == domain.StatusPublished
		changed, err := item.Publish()
		if err != nil || !changed {
			return err
		}
		if err := h.repo.Publish(txCtx, item); err != nil {
			return err
		}
		if wasPublished {
			return nil
		}
		mediaType, err := h.repo.MediaType(txCtx, item.MediaObjectID)
		if err != nil {
			return err
		}
		return h.publisher.Publish(txCtx, contract.EventPublished, item.ID.String(), map[string]string{
			"id":            item.ID.String(),
			"authorId":      item.AuthorID.String(),
			"mediaObjectId": item.MediaObjectID.String(),
			"mediaType":     mediaType,
		})
	})
	if err != nil {
		return ports.EditorReview{}, fmt.Errorf("publish review: %w", err)
	}
	return h.reader.Editor(ctx, reviewID, authorID)
}
