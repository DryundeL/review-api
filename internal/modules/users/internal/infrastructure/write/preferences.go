package write

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
)

func (r *Repository) GetContentPreferences(ctx context.Context, userID uuid.UUID) (domain.ContentPreferences, error) {
	out := domain.ContentPreferences{Types: []string{}, NeedsOnboarding: true}
	err := r.queryRow(ctx, `SELECT types FROM user_content_preferences WHERE user_id = $1`, userID).Scan(&out.Types)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("get content preferences: %w", err)
	}
	out.NeedsOnboarding = false
	return out, nil
}

func (r *Repository) SaveContentPreferences(ctx context.Context, userID uuid.UUID, types []string) (domain.ContentPreferences, error) {
	const sql = `INSERT INTO user_content_preferences (user_id, types)
 SELECT $1, $2::text[] WHERE (SELECT count(*) FROM review_types WHERE code = ANY($2::text[]) AND is_active) = cardinality($2::text[])
 ON CONFLICT (user_id) DO UPDATE SET types = EXCLUDED.types, updated_at = NOW()
 RETURNING types`
	var out domain.ContentPreferences
	err := r.queryRow(ctx, sql, userID, types).Scan(&out.Types)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, contract.FieldError{Field: "types"}
	}
	if err != nil {
		return out, fmt.Errorf("save content preferences: %w", err)
	}
	return out, nil
}
