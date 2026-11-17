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

type SaveDraftHandler struct {
	repo   ports.ReviewWriter
	reader ports.ReviewReader
	tx     transaction.Manager
}

func NewSaveDraftHandler(repo ports.ReviewWriter, reader ports.ReviewReader, tx transaction.Manager) *SaveDraftHandler {
	return &SaveDraftHandler{repo: repo, reader: reader, tx: tx}
}

// Handle autosaves the working version. It never touches the public version and emits no events.
func (h *SaveDraftHandler) Handle(ctx context.Context, reviewID, authorID uuid.UUID, revision int64, draft domain.Draft) (ports.EditorReview, error) {
	if h == nil || h.repo == nil || h.reader == nil || h.tx == nil {
		return ports.EditorReview{}, contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		item, err := lockOwnReview(txCtx, h.repo, reviewID, authorID, revision)
		if err != nil {
			return err
		}
		if err := item.SaveDraft(draft); err != nil {
			return err
		}
		return h.repo.SaveDraft(txCtx, item)
	})
	if err != nil {
		return ports.EditorReview{}, fmt.Errorf("save review draft: %w", err)
	}
	return h.reader.Editor(ctx, reviewID, authorID)
}

// lockOwnReview locks the row and checks owner and expected revision. Foreign reviews are 404.
func lockOwnReview(ctx context.Context, repo ports.ReviewWriter, reviewID, authorID uuid.UUID, revision int64) (domain.Review, error) {
	item, err := repo.GetForUpdate(ctx, reviewID)
	if err != nil {
		return domain.Review{}, err
	}
	if item.AuthorID != authorID {
		return domain.Review{}, contract.ErrNotFound
	}
	if item.Revision != revision {
		return domain.Review{}, contract.ErrRevisionConflict
	}
	return item, nil
}
