package read

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListItems(ctx context.Context, activeOnly bool) ([]ports.ShopItem, error) {
	q := `
		SELECT id, code, kind, title, price, is_active, sort_order
		FROM shop_items
	`
	if activeOnly {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY sort_order ASC, code ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list shop items: %w", err)
	}
	defer rows.Close()
	out := make([]ports.ShopItem, 0)
	for rows.Next() {
		var item ports.ShopItem
		if err := rows.Scan(&item.ID, &item.Code, &item.Kind, &item.Title, &item.Price, &item.IsActive, &item.SortOrder); err != nil {
			return nil, fmt.Errorf("scan shop item: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetMe(ctx context.Context, userID uuid.UUID) (ports.Me, error) {
	const ensure = `
		INSERT INTO gamification_profiles (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := r.pool.Exec(ctx, ensure, userID); err != nil {
		return ports.Me{}, fmt.Errorf("ensure profile: %w", err)
	}
	const q = `
		SELECT user_id, points_balance,
			COALESCE(equipped_background, ''),
			COALESCE(equipped_avatar_frame, ''),
			COALESCE(equipped_backdrop, '')
		FROM gamification_profiles
		WHERE user_id = $1
	`
	var me ports.Me
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&me.UserID, &me.PointsBalance, &me.EquippedBackground, &me.EquippedAvatarFrame, &me.EquippedBackdrop,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Me{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.Me{}, fmt.Errorf("get me: %w", err)
	}
	const owned = `
		SELECT i.code
		FROM shop_purchases p
		INNER JOIN shop_items i ON i.id = p.item_id
		WHERE p.user_id = $1
		ORDER BY i.sort_order ASC, i.code ASC
	`
	rows, err := r.pool.Query(ctx, owned, userID)
	if err != nil {
		return ports.Me{}, fmt.Errorf("list owned: %w", err)
	}
	defer rows.Close()
	me.OwnedCodes = make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return ports.Me{}, fmt.Errorf("scan owned: %w", err)
		}
		me.OwnedCodes = append(me.OwnedCodes, code)
	}
	return me, rows.Err()
}

func (r *Repository) ListAchievements(ctx context.Context, activeOnly bool) ([]ports.AchievementItem, error) {
	q := `
		SELECT id, code, title, description, metric, target, reward_points, is_active, sort_order
		FROM achievements
	`
	if activeOnly {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY sort_order ASC, code ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list achievements: %w", err)
	}
	defer rows.Close()
	out := make([]ports.AchievementItem, 0)
	for rows.Next() {
		var item ports.AchievementItem
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Title, &item.Description, &item.Metric,
			&item.Target, &item.RewardPoints, &item.IsActive, &item.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) ListProgress(ctx context.Context, userID uuid.UUID) ([]ports.AchievementProgress, error) {
	const q = `
		SELECT a.id, a.code, a.title, a.description, a.metric, a.target, a.reward_points, a.is_active, a.sort_order,
			COALESCE(m.value, 0),
			ua.unlocked_at
		FROM achievements a
		LEFT JOIN user_achievement_metrics m
			ON m.user_id = $1 AND m.metric = a.metric
		LEFT JOIN user_achievements ua
			ON ua.user_id = $1 AND ua.achievement_id = a.id
		WHERE a.is_active = TRUE
		ORDER BY a.sort_order ASC, a.code ASC
	`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list achievement progress: %w", err)
	}
	defer rows.Close()
	out := make([]ports.AchievementProgress, 0)
	for rows.Next() {
		var item ports.AchievementProgress
		var unlockedAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Title, &item.Description, &item.Metric,
			&item.Target, &item.RewardPoints, &item.IsActive, &item.SortOrder,
			&item.Current, &unlockedAt,
		); err != nil {
			return nil, fmt.Errorf("scan achievement progress: %w", err)
		}
		if unlockedAt != nil {
			s := unlockedAt.UTC().Format(time.RFC3339)
			item.UnlockedAt = &s
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetCosmetics(ctx context.Context, userID uuid.UUID) (ports.Profile, error) {
	const q = `
		SELECT user_id, points_balance,
			COALESCE(equipped_background, ''),
			COALESCE(equipped_avatar_frame, ''),
			COALESCE(equipped_backdrop, '')
		FROM gamification_profiles
		WHERE user_id = $1
	`
	var p ports.Profile
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&p.UserID, &p.PointsBalance, &p.EquippedBackground, &p.EquippedAvatarFrame, &p.EquippedBackdrop,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Profile{UserID: userID}, nil
	}
	if err != nil {
		return ports.Profile{}, fmt.Errorf("get cosmetics: %w", err)
	}
	return p, nil
}
