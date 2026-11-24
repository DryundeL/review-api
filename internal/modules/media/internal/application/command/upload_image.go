package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/ports"
	"review-api/internal/modules/media/internal/domain"
)

type UploadImageHandler struct {
	repo  ports.ImageWriter
	files ports.FileStore
	quota ports.Quota
}

func NewUploadImageHandler(repo ports.ImageWriter, files ports.FileStore, quota ports.Quota) *UploadImageHandler {
	return &UploadImageHandler{repo: repo, files: files, quota: quota}
}

// Handle validates the bytes, writes the cleaned file and records it.
// The quota is charged before validation, so rejected uploads also count.
func (h *UploadImageHandler) Handle(ctx context.Context, ownerID uuid.UUID, data []byte) (ports.ImageRecord, error) {
	if h == nil || h.repo == nil || h.files == nil || h.quota == nil || ownerID == uuid.Nil {
		return ports.ImageRecord{}, contract.ErrInvalid
	}
	ok, err := h.quota.Allow(ctx, ownerID)
	if err != nil {
		return ports.ImageRecord{}, fmt.Errorf("upload quota: %w", err)
	}
	if !ok {
		return ports.ImageRecord{}, contract.ErrQuotaExceeded
	}
	img, err := domain.Process(data)
	if err != nil {
		return ports.ImageRecord{}, err
	}

	id := uuid.New()
	rec := ports.ImageRecord{
		ID:          id,
		OwnerID:     ownerID,
		StorageKey:  "images/" + id.String() + "." + img.Format.Ext(),
		ContentType: string(img.Format),
		SizeBytes:   len(img.Data),
		Width:       img.Width,
		Height:      img.Height,
		CreatedAt:   time.Now().UTC(),
	}
	if err := h.files.Put(ctx, rec.StorageKey, img.Data); err != nil {
		return ports.ImageRecord{}, fmt.Errorf("store image: %w", err)
	}
	if err := h.repo.Insert(ctx, rec); err != nil {
		// Without a row the file is unreachable and never collected: remove it now.
		_ = h.files.Delete(context.WithoutCancel(ctx), rec.StorageKey)
		return ports.ImageRecord{}, err
	}
	return rec, nil
}
