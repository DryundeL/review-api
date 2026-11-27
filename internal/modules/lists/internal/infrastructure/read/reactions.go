package read

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
)

// openResultSQL matches results others may see and react to; it is the same rule as
// query.CanViewResult for a viewer who is not the author.
const openResultSQL = `
	SELECT 1 FROM list_results r
	JOIN list_templates t ON t.id = r.template_id
	WHERE r.id = $1 AND r.status = 'published' AND r.deleted_at IS NULL
		AND r.moderation_hidden_at IS NULL AND t.moderation_hidden_at IS NULL`

const commentSelect = `
	SELECT c.id, c.result_id, c.author_id, c.parent_id, c.body, c.created_at,
		COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
		EXISTS (SELECT 1 FROM list_result_comments child WHERE child.result_id = c.result_id AND child.parent_id = c.id)
	FROM list_result_comments c
	JOIN users u ON u.id = c.author_id
	WHERE EXISTS (` + openResultSQL + `)
		AND c.result_id = $1`

func (r *Repository) ViewerVote(ctx context.Context, resultID, viewerID uuid.UUID) (*bool, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	if viewerID == uuid.Nil {
		return nil, nil
	}
	var isLike bool
	err := r.pool.QueryRow(ctx, `SELECT is_like FROM list_result_votes WHERE result_id = $1 AND user_id = $2`,
		resultID, viewerID).Scan(&isLike)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read list result vote: %w", err)
	}
	return &isLike, nil
}

func scanComment(row pgx.Row) (ports.Comment, error) {
	var c ports.Comment
	err := row.Scan(&c.ID, &c.ResultID, &c.AuthorID, &c.ParentID, &c.Body, &c.CreatedAt,
		&c.Author.Username, &c.Author.PhotoURL, &c.HasReplies)
	c.Author.ID = c.AuthorID
	return c, err
}

func (r *Repository) Comment(ctx context.Context, resultID, commentID uuid.UUID) (ports.Comment, error) {
	if r == nil || r.pool == nil {
		return ports.Comment{}, contract.ErrInvalid
	}
	c, err := scanComment(r.pool.QueryRow(ctx, commentSelect+` AND c.id = $2`, resultID, commentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Comment{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.Comment{}, fmt.Errorf("read list comment: %w", err)
	}
	return c, nil
}

func (r *Repository) ListComments(ctx context.Context, resultID uuid.UUID, parentID *uuid.UUID, after *ports.CommentPosition, limit int) ([]ports.Comment, error) {
	if r == nil || r.pool == nil || limit < 1 {
		return nil, contract.ErrInvalid
	}
	// An empty page must still tell a missing result or parent from a thread without comments.
	if parentID != nil {
		if _, err := r.Comment(ctx, resultID, *parentID); err != nil {
			return nil, err
		}
	} else {
		var open bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS (`+openResultSQL+`)`, resultID).Scan(&open); err != nil {
			return nil, fmt.Errorf("read list result: %w", err)
		}
		if !open {
			return nil, contract.ErrNotFound
		}
	}
	// Two fixed query shapes, so that both use list_result_comments_page_idx ("IS NOT DISTINCT FROM"
	// would not). $3/$4 are the cursor, NULL on the first page; $5 is the parent.
	var afterT, afterID any
	if after != nil {
		afterT, afterID = after.CreatedAt, after.ID
	}
	args := []any{resultID, limit, afterT, afterID}
	parentFilter := ` AND c.parent_id IS NULL`
	if parentID != nil {
		args = append(args, *parentID)
		parentFilter = ` AND c.parent_id = $5`
	}
	sql := commentSelect + parentFilter + `
		AND ($3::timestamptz IS NULL OR (c.created_at, c.id) > ($3::timestamptz, $4::uuid))
	ORDER BY c.created_at, c.id
	LIMIT $2`
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list list comments: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Comment, 0, limit)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan list comment: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list list comments: %w", err)
	}
	return out, nil
}
