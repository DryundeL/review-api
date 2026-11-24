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

// DefaultResultQuota is used when the module is built without an explicit limit.
const DefaultResultQuota = 200

// ResultHandler holds the author actions on results. Every write runs under the result
// row lock; creation and copy also take the author's quota lock.
type ResultHandler struct {
	repo  ports.ResultWriter
	tx    transaction.Manager
	quota int
}

func NewResultHandler(repo ports.ResultWriter, tx transaction.Manager, quota int) *ResultHandler {
	if quota < 1 {
		quota = DefaultResultQuota
	}
	return &ResultHandler{repo: repo, tx: tx, quota: quota}
}

// Create starts an empty draft on the snapshot of the template's current revision.
// Any published template the viewer can open accepts results, unlisted ones included.
func (h *ResultHandler) Create(ctx context.Context, templateID, authorID uuid.UUID) (domain.Result, error) {
	if h == nil || h.repo == nil || h.tx == nil || authorID == uuid.Nil {
		return domain.Result{}, contract.ErrInvalid
	}
	var res domain.Result
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.checkQuota(txCtx, authorID); err != nil {
			return err
		}
		// FOR SHARE: the working state cannot change while it is copied into the snapshot.
		tpl, err := h.repo.GetTemplateForShare(txCtx, templateID)
		if err != nil {
			return err
		}
		if !tpl.AcceptsResults() {
			return templateRefusal(tpl, authorID)
		}
		snap, err := h.repo.EnsureSnapshot(txCtx, tpl)
		if err != nil {
			return err
		}
		res, err = domain.NewResult(uuid.New(), authorID, snap, time.Now())
		if err != nil {
			return err
		}
		return h.repo.InsertResult(txCtx, res)
	})
	if err != nil {
		return domain.Result{}, fmt.Errorf("create list result: %w", err)
	}
	return res, nil
}

// templateRefusal hides unavailable foreign templates and explains the refusal to their author.
func templateRefusal(tpl domain.Template, viewerID uuid.UUID) error {
	if tpl.AuthorID != viewerID {
		return contract.ErrNotFound
	}
	if tpl.ModerationHiddenAt != nil {
		return contract.ErrModerated
	}
	return contract.ErrConflict
}

type EntryInput struct {
	ItemID   uuid.UUID
	TierID   *uuid.UUID
	Position int
}

// SaveResultInput is a full replacement of the author's fields and the placement.
type SaveResultInput struct {
	Revision    int
	Title       string
	Description string
	Entries     []EntryInput
}

// Save replaces the result and bumps its revision. It works whatever has happened to the
// template since: the result is bound to its snapshot.
func (h *ResultHandler) Save(ctx context.Context, id, authorID uuid.UUID, in SaveResultInput) (domain.Result, error) {
	if len(in.Entries) > domain.MaxItems {
		return domain.Result{}, contract.Field("entries", contract.ReasonTooMany)
	}
	entries := make([]domain.Entry, len(in.Entries))
	for i, e := range in.Entries {
		entries[i] = domain.Entry{ItemID: e.ItemID, TierID: e.TierID, Position: e.Position}
	}
	return h.update(ctx, "save", id, authorID, func(txCtx context.Context, res *domain.Result) (bool, error) {
		if res.Revision != in.Revision {
			return false, contract.ErrRevisionConflict
		}
		snap, err := h.repo.Snapshot(txCtx, res.VersionID)
		if err != nil {
			return false, err
		}
		if err := res.Replace(snap, in.Title, in.Description, entries); err != nil {
			return false, err
		}
		res.Revision++
		return true, nil
	})
}

// Publish publishes the saved revision the author has seen. A deleted or unpublished template
// does not block it; a template hidden by a moderator does.
func (h *ResultHandler) Publish(ctx context.Context, id, authorID uuid.UUID, revision int, visibility string) (domain.Result, error) {
	vis, err := domain.ParseVisibility(visibility)
	if err != nil {
		return domain.Result{}, err
	}
	return h.update(ctx, "publish", id, authorID, func(txCtx context.Context, res *domain.Result) (bool, error) {
		if res.Revision != revision {
			return false, contract.ErrRevisionConflict
		}
		moderated, err := h.repo.TemplateModerated(txCtx, res.TemplateID)
		if err != nil {
			return false, err
		}
		return false, res.Publish(vis, time.Now(), moderated)
	})
}

func (h *ResultHandler) Unpublish(ctx context.Context, id, authorID uuid.UUID) (domain.Result, error) {
	return h.update(ctx, "unpublish", id, authorID, func(_ context.Context, res *domain.Result) (bool, error) {
		res.Unpublish()
		return false, nil
	})
}

func (h *ResultHandler) SetVisibility(ctx context.Context, id, authorID uuid.UUID, visibility string) (domain.Result, error) {
	if visibility == "" {
		return domain.Result{}, contract.Field("visibility", contract.ReasonRequired)
	}
	vis, err := domain.ParseVisibility(visibility)
	if err != nil {
		return domain.Result{}, err
	}
	return h.update(ctx, "set visibility of", id, authorID, func(_ context.Context, res *domain.Result) (bool, error) {
		return false, res.SetVisibility(vis)
	})
}

// Delete is a soft delete: the result disappears for everyone, including its author.
func (h *ResultHandler) Delete(ctx context.Context, id, authorID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil {
		return contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := h.lockOwn(txCtx, id, authorID); err != nil {
			return err
		}
		return h.repo.SoftDeleteResult(txCtx, id)
	})
	if err != nil {
		return fmt.Errorf("delete list result: %w", err)
	}
	return nil
}

// Copy duplicates the author's result with its placement on the same snapshot, even when the
// template is deleted or unpublished. Moderation of the result or its template blocks it.
func (h *ResultHandler) Copy(ctx context.Context, id, authorID uuid.UUID) (domain.Result, error) {
	if h == nil || h.repo == nil || h.tx == nil {
		return domain.Result{}, contract.ErrInvalid
	}
	var cp domain.Result
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.checkQuota(txCtx, authorID); err != nil {
			return err
		}
		src, err := h.lockOwn(txCtx, id, authorID)
		if err != nil {
			return err
		}
		moderated, err := h.repo.TemplateModerated(txCtx, src.TemplateID)
		if err != nil {
			return err
		}
		if cp, err = src.Copy(uuid.New(), time.Now(), moderated); err != nil {
			return err
		}
		return h.repo.InsertResult(txCtx, cp)
	})
	if err != nil {
		return domain.Result{}, fmt.Errorf("copy list result: %w", err)
	}
	return cp, nil
}

// checkQuota must run inside the inserting transaction, see checkTemplateQuota.
func (h *ResultHandler) checkQuota(ctx context.Context, authorID uuid.UUID) error {
	if err := h.repo.LockAuthor(ctx, authorID); err != nil {
		return err
	}
	n, err := h.repo.CountResults(ctx, authorID)
	if err != nil {
		return err
	}
	if n >= h.quota {
		return contract.ErrQuotaExceeded
	}
	return nil
}

// lockOwn locks a result of the author. Foreign and deleted results are 404.
func (h *ResultHandler) lockOwn(ctx context.Context, id, authorID uuid.UUID) (domain.Result, error) {
	res, err := h.repo.GetResultForUpdate(ctx, id)
	if err != nil {
		return domain.Result{}, err
	}
	if res.AuthorID != authorID {
		return domain.Result{}, contract.ErrNotFound
	}
	return res, nil
}

// update locks the author's result, applies fn and stores it (with the entries when fn says so).
func (h *ResultHandler) update(ctx context.Context, action string, id, authorID uuid.UUID, fn func(context.Context, *domain.Result) (bool, error)) (domain.Result, error) {
	if h == nil || h.repo == nil || h.tx == nil {
		return domain.Result{}, contract.ErrInvalid
	}
	var saved domain.Result
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		res, err := h.lockOwn(txCtx, id, authorID)
		if err != nil {
			return err
		}
		withEntries, err := fn(txCtx, &res)
		if err != nil {
			return err
		}
		if err := h.repo.UpdateResult(txCtx, res, withEntries); err != nil {
			return err
		}
		saved = res
		return nil
	})
	if err != nil {
		return domain.Result{}, fmt.Errorf("%s list result: %w", action, err)
	}
	return saved, nil
}
