package command

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/ports"
)

// AttachImagesHandler is called by other modules inside their write transaction.
// Row locks taken by the UPDATE serialize it with the GC, so an attached image keeps its file.
type AttachImagesHandler struct {
	repo         ports.ImageWriter
	publicPrefix string
}

func NewAttachImagesHandler(repo ports.ImageWriter, publicPrefix string) *AttachImagesHandler {
	return &AttachImagesHandler{repo: repo, publicPrefix: strings.TrimRight(publicPrefix, "/")}
}

// Handle returns refs in the order of the first occurrence of each id.
func (h *AttachImagesHandler) Handle(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]contract.ImageRef, error) {
	if h == nil || h.repo == nil || ownerID == uuid.Nil {
		return nil, contract.ErrInvalid
	}
	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, contract.ErrNotFound
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, nil
	}
	if len(unique) > contract.MaxAttachBatch {
		return nil, contract.ErrInvalid
	}
	records, err := h.repo.Attach(ctx, ownerID, unique)
	if err != nil {
		return nil, err
	}
	if len(records) != len(unique) {
		return nil, contract.ErrNotFound
	}
	byID := make(map[uuid.UUID]ports.ImageRecord, len(records))
	for _, rec := range records {
		byID[rec.ID] = rec
	}
	out := make([]contract.ImageRef, 0, len(unique))
	for _, id := range unique {
		rec, ok := byID[id]
		if !ok {
			return nil, contract.ErrNotFound
		}
		out = append(out, contract.ImageRef{ID: rec.ID, URL: h.publicPrefix + "/" + rec.StorageKey, Width: rec.Width, Height: rec.Height})
	}
	return out, nil
}
