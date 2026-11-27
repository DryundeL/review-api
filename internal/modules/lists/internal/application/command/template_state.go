package command

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/platform/transaction"
)

// TemplateStateHandler groups the short author actions on a template: publish, unpublish,
// visibility, delete and refresh of a catalog item. Each runs under the template row lock.
type TemplateStateHandler struct {
	repo    ports.TemplateWriter
	catalog ports.Catalog
	tx      transaction.Manager
}

func NewTemplateStateHandler(repo ports.TemplateWriter, catalog ports.Catalog, tx transaction.Manager) *TemplateStateHandler {
	return &TemplateStateHandler{repo: repo, catalog: catalog, tx: tx}
}

// Publish publishes exactly the saved revision the author has seen.
func (h *TemplateStateHandler) Publish(ctx context.Context, id, authorID uuid.UUID, revision int, visibility string) (domain.Template, error) {
	vis, err := domain.ParseVisibility(visibility)
	if err != nil {
		return domain.Template{}, err
	}
	return h.update(ctx, "publish", id, authorID, func(_ context.Context, tpl *domain.Template) (bool, error) {
		if tpl.Revision != revision {
			return false, contract.ErrRevisionConflict
		}
		return false, tpl.Publish(vis, time.Now())
	})
}

func (h *TemplateStateHandler) Unpublish(ctx context.Context, id, authorID uuid.UUID) (domain.Template, error) {
	return h.update(ctx, "unpublish", id, authorID, func(_ context.Context, tpl *domain.Template) (bool, error) {
		tpl.Unpublish()
		return false, nil
	})
}

func (h *TemplateStateHandler) SetVisibility(ctx context.Context, id, authorID uuid.UUID, visibility string) (domain.Template, error) {
	if visibility == "" {
		return domain.Template{}, contract.Field("visibility", contract.ReasonRequired)
	}
	vis, err := domain.ParseVisibility(visibility)
	if err != nil {
		return domain.Template{}, err
	}
	return h.update(ctx, "set visibility of", id, authorID, func(_ context.Context, tpl *domain.Template) (bool, error) {
		return false, tpl.SetVisibility(vis)
	})
}

// RefreshItem copies the current title and poster of the item's catalog work into the template.
// A missing poster keeps the current image. Earlier results keep their snapshots.
func (h *TemplateStateHandler) RefreshItem(ctx context.Context, id, itemID, authorID uuid.UUID, revision int) (domain.Template, error) {
	return h.update(ctx, "refresh item of", id, authorID, func(txCtx context.Context, tpl *domain.Template) (bool, error) {
		if tpl.Revision != revision {
			return false, contract.ErrRevisionConflict
		}
		c := tpl.Content()
		c.Items = slices.Clone(c.Items)
		i := slices.IndexFunc(c.Items, func(it domain.Item) bool { return it.ID == itemID })
		if i < 0 {
			return false, contract.ErrNotFound
		}
		if c.Items[i].MediaObjectID == nil {
			return false, contract.ErrConflict
		}
		work, err := h.catalog.Work(txCtx, *c.Items[i].MediaObjectID)
		if errors.Is(err, contract.ErrNotFound) {
			return false, contract.ErrConflict
		}
		if err != nil {
			return false, err
		}
		c.Items[i].Title = work.Title
		if work.PosterURL != "" {
			c.Items[i].ImageID, c.Items[i].ImageURL = nil, work.PosterURL
		}
		if err := tpl.Replace(c); err != nil {
			return false, err
		}
		tpl.Revision++
		return true, nil
	})
}

// Delete is a soft delete: the template disappears for everyone, results keep their snapshots.
func (h *TemplateStateHandler) Delete(ctx context.Context, id, authorID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil {
		return contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := lockOwnTemplate(txCtx, h.repo, id, authorID); err != nil {
			return err
		}
		return h.repo.SoftDeleteTemplate(txCtx, id)
	})
	if err != nil {
		return fmt.Errorf("delete list template: %w", err)
	}
	return nil
}

// update locks the author's template, applies fn and stores the header (and the content when fn says so).
func (h *TemplateStateHandler) update(ctx context.Context, action string, id, authorID uuid.UUID, fn func(context.Context, *domain.Template) (bool, error)) (domain.Template, error) {
	if h == nil || h.repo == nil || h.catalog == nil || h.tx == nil {
		return domain.Template{}, contract.ErrInvalid
	}
	var saved domain.Template
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		tpl, err := lockOwnTemplate(txCtx, h.repo, id, authorID)
		if err != nil {
			return err
		}
		withContent, err := fn(txCtx, &tpl)
		if err != nil {
			return err
		}
		if err := h.repo.UpdateTemplate(txCtx, tpl, withContent); err != nil {
			return err
		}
		saved = tpl
		return nil
	})
	if err != nil {
		return domain.Template{}, fmt.Errorf("%s list template: %w", action, err)
	}
	return saved, nil
}

type CopyTemplateHandler struct {
	repo  ports.TemplateWriter
	tx    transaction.Manager
	quota int
}

func NewCopyTemplateHandler(repo ports.TemplateWriter, tx transaction.Manager, quota int) *CopyTemplateHandler {
	if quota < 1 {
		quota = DefaultTemplateQuota
	}
	return &CopyTemplateHandler{repo: repo, tx: tx, quota: quota}
}

// Handle copies the author's own template into a new draft. Moderated templates cannot be copied.
func (h *CopyTemplateHandler) Handle(ctx context.Context, id, authorID uuid.UUID) (domain.Template, error) {
	if h == nil || h.repo == nil || h.tx == nil {
		return domain.Template{}, contract.ErrInvalid
	}
	var cp domain.Template
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := checkTemplateQuota(txCtx, h.repo, authorID, h.quota); err != nil {
			return err
		}
		src, err := lockOwnTemplate(txCtx, h.repo, id, authorID)
		if err != nil {
			return err
		}
		cp, err = src.Copy(uuid.New(), time.Now())
		if err != nil {
			return err
		}
		return h.repo.InsertTemplate(txCtx, cp)
	})
	if err != nil {
		return domain.Template{}, fmt.Errorf("copy list template: %w", err)
	}
	return cp, nil
}
