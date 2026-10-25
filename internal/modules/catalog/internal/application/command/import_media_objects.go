package command

import (
	"context"
	"errors"
	"fmt"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/ports"
	"review-api/internal/modules/catalog/internal/domain"
	"review-api/internal/platform/transaction"
)

type ImportMediaObjectsHandler struct {
	repo      ports.Writer
	tx        transaction.Manager
	types     ports.TypeChecker
	publisher ports.Publisher
}

func NewImportMediaObjectsHandler(repo ports.Writer, tx transaction.Manager, types ports.TypeChecker, publisher ports.Publisher) *ImportMediaObjectsHandler {
	return &ImportMediaObjectsHandler{repo: repo, tx: tx, types: types, publisher: publisher}
}

func (h *ImportMediaObjectsHandler) Handle(ctx context.Context, drafts []domain.Draft) ([]ports.MediaObject, error) {
	if h == nil || h.repo == nil || h.tx == nil || h.types == nil || h.publisher == nil {
		return nil, contract.ErrInvalid
	}
	if len(drafts) == 0 || len(drafts) > domain.MaxImportBatch() {
		return nil, contract.FieldError{Field: "items", Index: -1}
	}

	items := make([]domain.MediaObject, 0, len(drafts))
	typeSet := make(map[string]struct{})
	for i, draft := range drafts {
		item, err := domain.NewImported(draft)
		if err != nil {
			var field contract.FieldError
			if errors.As(err, &field) {
				field.Index = i
				return nil, field
			}
			return nil, err
		}
		items = append(items, item)
		typeSet[item.Type] = struct{}{}
	}

	codes := make([]string, 0, len(typeSet))
	for code := range typeSet {
		codes = append(codes, code)
	}
	if err := h.types.RequireActive(ctx, codes); err != nil {
		var field contract.FieldError
		if errors.As(err, &field) {
			field.Index = -1
			return nil, field
		}
		return nil, err
	}

	out := make([]ports.MediaObject, 0, len(items))
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		for _, item := range items {
			if err := h.repo.Save(txCtx, item); err != nil {
				return err
			}
			if err := h.publisher.Publish(txCtx, contract.EventCreated, item.ID.String(), map[string]string{
				"id":    item.ID.String(),
				"title": item.Title,
				"type":  item.Type,
			}); err != nil {
				return err
			}
			out = append(out, toPort(item))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("import media objects: %w", err)
	}
	return out, nil
}

func toPort(item domain.MediaObject) ports.MediaObject {
	return ports.MediaObject{
		ID:             item.ID,
		Title:          item.Title,
		OriginalTitle:  item.OriginalTitle,
		Type:           item.Type,
		Info:           item.Info,
		PosterURL:      item.PosterURL,
		ExternalSource: item.ExternalSource,
		ExternalID:     item.ExternalID,
		CreatedBy:      item.CreatedBy,
	}
}
