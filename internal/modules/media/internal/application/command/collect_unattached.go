package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/ports"
	"review-api/internal/platform/transaction"
)

// CollectUnattachedHandler removes uploads that no module attached within ttl.
//
// Rows stay locked while files are removed, so a concurrent attach either waits and then
// finds the row gone, or attaches first and the row is skipped. A file whose delete fails
// keeps its row and is retried on the next run.
type CollectUnattachedHandler struct {
	repo  ports.ImageWriter
	files ports.FileStore
	tx    transaction.Manager
	ttl   time.Duration
	now   func() time.Time
}

func NewCollectUnattachedHandler(repo ports.ImageWriter, files ports.FileStore, tx transaction.Manager, ttl time.Duration) *CollectUnattachedHandler {
	return &CollectUnattachedHandler{repo: repo, files: files, tx: tx, ttl: ttl, now: time.Now}
}

// Handle processes one batch and returns how many images were removed.
func (h *CollectUnattachedHandler) Handle(ctx context.Context, batch int) (int, error) {
	if h == nil || h.repo == nil || h.files == nil || h.tx == nil || h.ttl <= 0 || batch < 1 {
		return 0, contract.ErrInvalid
	}
	cutoff := h.now().UTC().Add(-h.ttl)
	removed := 0
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		items, err := h.repo.LockUnattached(txCtx, cutoff, batch)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, 0, len(items))
		for _, it := range items {
			if err := h.files.Delete(txCtx, it.StorageKey); err != nil {
				slog.WarnContext(ctx, "media gc: file delete failed", "image_id", it.ID, "error", err)
				continue
			}
			ids = append(ids, it.ID)
		}
		if err := h.repo.DeleteByIDs(txCtx, ids); err != nil {
			return err
		}
		removed = len(ids)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("collect unattached images: %w", err)
	}
	return removed, nil
}
