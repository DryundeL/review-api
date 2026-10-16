package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

type CreateReviewHandler struct {
	repo      ports.ReviewWriter
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewCreateReviewHandler(repo ports.ReviewWriter, tx transaction.Manager, publisher ports.Publisher) *CreateReviewHandler {
	return &CreateReviewHandler{repo: repo, tx: tx, publisher: publisher}
}

func (h *CreateReviewHandler) Handle(ctx context.Context, authorID uuid.UUID, draft domain.Draft) (ports.Review, error) {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return ports.Review{}, contract.ErrInvalid
	}
	item, err := domain.NewReview(authorID, draft)
	if err != nil {
		return ports.Review{}, err
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Create(txCtx, item); err != nil {
			return err
		}
		if item.Status == domain.StatusPublished {
			return h.publisher.Publish(txCtx, contract.EventPublished, item.ID.String(), map[string]string{
				"id":            item.ID.String(),
				"authorId":      item.AuthorID.String(),
				"mediaObjectId": item.MediaObjectID.String(),
			})
		}
		return nil
	})
	if err != nil {
		return ports.Review{}, fmt.Errorf("create review: %w", err)
	}
	return toPortReview(item), nil
}

func toPortReview(item domain.Review) ports.Review {
	blocks := make([]ports.Block, 0, len(item.Blocks))
	hasSpoilers := false
	for _, b := range item.Blocks {
		blocks = append(blocks, ports.Block{
			ID: b.ID, Position: b.Position, Title: b.Title, Body: b.Body,
		})
		if !hasSpoilers {
			hasSpoilers = bodyHasSpoilers(b.Body)
		}
	}
	return ports.Review{
		ID:       item.ID,
		AuthorID: item.AuthorID,
		MediaObject: ports.MediaObjectRef{
			ID: item.MediaObjectID,
		},
		Title:       item.Title,
		Rating:      item.Rating,
		Status:      item.Status,
		Blocks:      blocks,
		HasSpoilers: hasSpoilers,
	}
}

func bodyHasSpoilers(body string) bool {
	i := strings.Index(body, "||")
	if i < 0 {
		return false
	}
	return strings.Contains(body[i+2:], "||")
}
