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

// ReactionHandler holds votes, views and comments on results, with the rules of reviews:
// only open results (published, not hidden; unlisted included) take reactions, and the
// author's own views are not counted. Anything else is the same 404 as a missing result.
type ReactionHandler struct {
	repo     ports.ReactionWriter
	comments ports.CommentReader
	tx       transaction.Manager
}

func NewReactionHandler(repo ports.ReactionWriter, comments ports.CommentReader, tx transaction.Manager) *ReactionHandler {
	return &ReactionHandler{repo: repo, comments: comments, tx: tx}
}

func (h *ReactionHandler) ready(userID uuid.UUID) error {
	if h == nil || h.repo == nil || h.comments == nil || h.tx == nil || userID == uuid.Nil {
		return contract.ErrInvalid
	}
	return nil
}

// Vote sets a like or a dislike; a repeated vote changes nothing.
func (h *ReactionHandler) Vote(ctx context.Context, resultID, userID uuid.UUID, isLike bool) error {
	if err := h.ready(userID); err != nil {
		return err
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := h.repo.LockOpenResult(txCtx, resultID, true); err != nil {
			return err
		}
		return h.repo.UpsertVote(txCtx, resultID, userID, isLike)
	})
	if err != nil {
		return fmt.Errorf("vote list result: %w", err)
	}
	return nil
}

// View counts the first view of each user other than the author.
func (h *ReactionHandler) View(ctx context.Context, resultID, userID uuid.UUID) error {
	if err := h.ready(userID); err != nil {
		return err
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		authorID, err := h.repo.LockOpenResult(txCtx, resultID, false)
		if err != nil {
			return err
		}
		if authorID == userID {
			return nil
		}
		return h.repo.InsertView(txCtx, resultID, userID)
	})
	if err != nil {
		return fmt.Errorf("view list result: %w", err)
	}
	return nil
}

// Comment adds a comment or, with parentID, a reply to a comment of the same result.
func (h *ReactionHandler) Comment(ctx context.Context, resultID, authorID uuid.UUID, parentID *uuid.UUID, body string) (ports.Comment, error) {
	if err := h.ready(authorID); err != nil {
		return ports.Comment{}, err
	}
	c, err := domain.NewComment(uuid.New(), resultID, authorID, parentID, body, time.Now())
	if err != nil {
		return ports.Comment{}, err
	}
	var out ports.Comment
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := h.repo.LockOpenResult(txCtx, resultID, false); err != nil {
			return err
		}
		if c.ParentID != nil {
			ok, err := h.repo.CommentExists(txCtx, resultID, *c.ParentID)
			if err != nil {
				return err
			}
			if !ok {
				return contract.ErrNotFound
			}
		}
		return h.repo.InsertComment(txCtx, c)
	})
	if err != nil {
		return ports.Comment{}, fmt.Errorf("comment list result: %w", err)
	}
	// Read after commit: the reader works on the pool, outside the transaction.
	out, err = h.comments.Comment(ctx, resultID, c.ID)
	if err != nil {
		return ports.Comment{}, fmt.Errorf("read list comment: %w", err)
	}
	return out, nil
}
