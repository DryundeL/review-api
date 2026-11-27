package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/platform/transaction"
)

// DefaultTemplateQuota is used when the module is built without an explicit limit.
const DefaultTemplateQuota = 50

type CreateTemplateInput struct {
	TypeCode string
	Format   string
	Title    string
}

type CreateTemplateHandler struct {
	repo  ports.TemplateWriter
	types ports.Types
	tx    transaction.Manager
	quota int
}

func NewCreateTemplateHandler(repo ports.TemplateWriter, types ports.Types, tx transaction.Manager, quota int) *CreateTemplateHandler {
	if quota < 1 {
		quota = DefaultTemplateQuota
	}
	return &CreateTemplateHandler{repo: repo, types: types, tx: tx, quota: quota}
}

// Handle creates an empty draft. The author's quota counts templates that are not deleted.
func (h *CreateTemplateHandler) Handle(ctx context.Context, authorID uuid.UUID, in CreateTemplateInput) (domain.Template, error) {
	if h == nil || h.repo == nil || h.types == nil || h.tx == nil || authorID == uuid.Nil {
		return domain.Template{}, contract.ErrInvalid
	}
	format, err := domain.ParseFormat(in.Format)
	if err != nil {
		return domain.Template{}, err
	}
	tpl, err := domain.NewTemplate(uuid.New(), authorID, in.TypeCode, format, in.Title, time.Now())
	if err != nil {
		return domain.Template{}, err
	}
	if tpl.TypeCode != domain.TypeMixed {
		if err := h.types.RequireActive(ctx, tpl.TypeCode); err != nil {
			return domain.Template{}, err
		}
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := checkTemplateQuota(txCtx, h.repo, authorID, h.quota); err != nil {
			return err
		}
		return h.repo.InsertTemplate(txCtx, tpl)
	})
	if err != nil {
		return domain.Template{}, fmt.Errorf("create list template: %w", err)
	}
	return tpl, nil
}

// checkTemplateQuota must run inside the transaction that inserts the template:
// the advisory lock keeps concurrent requests of one author from passing the limit together.
func checkTemplateQuota(ctx context.Context, repo ports.TemplateWriter, authorID uuid.UUID, quota int) error {
	if err := repo.LockAuthor(ctx, authorID); err != nil {
		return err
	}
	n, err := repo.CountTemplates(ctx, authorID)
	if err != nil {
		return err
	}
	if n >= quota {
		return contract.ErrQuotaExceeded
	}
	return nil
}

// lockOwnTemplate locks a template of the author. Foreign and deleted templates are 404.
func lockOwnTemplate(ctx context.Context, repo ports.TemplateWriter, id, authorID uuid.UUID) (domain.Template, error) {
	tpl, err := repo.GetTemplateForUpdate(ctx, id)
	if err != nil {
		return domain.Template{}, err
	}
	if tpl.AuthorID != authorID {
		return domain.Template{}, contract.ErrNotFound
	}
	return tpl, nil
}
