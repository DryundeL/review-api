package command

import (
	"context"
	"fmt"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/ports"
	"review-api/internal/modules/catalog/internal/domain"
	"review-api/internal/platform/transaction"
)

type CreateWorkHandler struct {
	repo      ports.Writer
	tx        transaction.Manager
	types     ports.TypeChecker
	authors   ports.AuthorResolver
	publisher ports.Publisher
}

func NewCreateWorkHandler(repo ports.Writer, tx transaction.Manager, types ports.TypeChecker, authors ports.AuthorResolver, publisher ports.Publisher) *CreateWorkHandler {
	return &CreateWorkHandler{repo: repo, tx: tx, types: types, authors: authors, publisher: publisher}
}

func (h *CreateWorkHandler) Handle(ctx context.Context, telegramID int64, draft domain.Draft) (ports.Work, error) {
	if h == nil || h.repo == nil || h.tx == nil || h.types == nil || h.authors == nil || h.publisher == nil || telegramID <= 0 {
		return ports.Work{}, contract.ErrInvalid
	}
	author, err := h.authors.UserID(ctx, telegramID)
	if err != nil {
		return ports.Work{}, err
	}
	work, err := domain.NewUserWork(author, draft)
	if err != nil {
		return ports.Work{}, err
	}
	if err := h.types.RequireActive(ctx, work.Types); err != nil {
		return ports.Work{}, err
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, work); err != nil {
			return err
		}
		return h.publisher.Publish(txCtx, contract.EventCreated, work.ID.String(), map[string]string{
			"id":          work.ID.String(),
			"title":       work.Title,
			"primaryType": work.PrimaryType,
			"origin":      work.Origin,
		})
	})
	if err != nil {
		return ports.Work{}, fmt.Errorf("create work: %w", err)
	}
	return toPort(work), nil
}

func toPort(work domain.Work) ports.Work {
	return ports.Work{
		ID:            work.ID,
		Title:         work.Title,
		PrimaryType:   work.PrimaryType,
		Types:         append([]string(nil), work.Types...),
		ReleaseYear:   work.ReleaseYear,
		CoverURL:      work.CoverURL,
		ReleaseStatus: work.ReleaseStatus,
		Origin:        work.Origin,
		CreatedBy:     work.CreatedBy,
	}
}
