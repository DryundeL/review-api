package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
	"review-api/internal/platform/transaction"
)

const maxSort = 10000

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateItem(ctx context.Context, item domain.ShopItem) error {
	const q = `
		INSERT INTO shop_items (id, code, kind, title, price, is_active, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if err := r.exec(ctx, q, item.ID, item.Code, item.Kind, item.Title, item.Price, item.IsActive, item.SortOrder); err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) UpdateItem(ctx context.Context, id uuid.UUID, title *string, price *int, active *bool, sortOrder *int) (ports.ShopItem, error) {
	const q = `
		UPDATE shop_items
		SET title = COALESCE($2, title),
			price = COALESCE($3, price),
			is_active = COALESCE($4, is_active),
			sort_order = COALESCE($5, sort_order),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, code, kind, title, price, is_active, sort_order
	`
	var item ports.ShopItem
	err := r.queryRow(ctx, q, id, title, price, active, sortOrder).Scan(
		&item.ID, &item.Code, &item.Kind, &item.Title, &item.Price, &item.IsActive, &item.SortOrder,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ShopItem{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.ShopItem{}, mapWriteError(err)
	}
	return item, nil
}

func (r *Repository) GetItem(ctx context.Context, id uuid.UUID) (ports.ShopItem, error) {
	const q = `
		SELECT id, code, kind, title, price, is_active, sort_order
		FROM shop_items WHERE id = $1
	`
	var item ports.ShopItem
	err := r.queryRow(ctx, q, id).Scan(
		&item.ID, &item.Code, &item.Kind, &item.Title, &item.Price, &item.IsActive, &item.SortOrder,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ShopItem{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.ShopItem{}, fmt.Errorf("get shop item: %w", err)
	}
	return item, nil
}

func (r *Repository) NextSort(ctx context.Context) (int, error) {
	const q = `SELECT COALESCE(MAX(sort_order), 0) + 10 FROM shop_items`
	var next int
	if err := r.queryRow(ctx, q).Scan(&next); err != nil {
		return 0, fmt.Errorf("next shop sort: %w", err)
	}
	if next > maxSort {
		return 0, contract.FieldError{Field: "sortOrder"}
	}
	return next, nil
}

func (r *Repository) EnsureProfile(ctx context.Context, userID uuid.UUID) error {
	const q = `
		INSERT INTO gamification_profiles (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if err := r.exec(ctx, q, userID); err != nil {
		return fmt.Errorf("ensure gamification profile: %w", err)
	}
	return nil
}

func (r *Repository) GrantPoints(ctx context.Context, userID uuid.UUID, amount int, reason, sourceKey string) (bool, error) {
	if amount < 1 || sourceKey == "" || reason == "" || userID == uuid.Nil {
		return false, contract.ErrInvalid
	}
	if err := r.EnsureProfile(ctx, userID); err != nil {
		return false, err
	}
	const insertGrant = `
		INSERT INTO point_grants (id, user_id, amount, reason, source_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (source_key) DO NOTHING
	`
	tag, err := r.execTag(ctx, insertGrant, uuid.New(), userID, amount, reason, sourceKey)
	if err != nil {
		return false, mapWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	const bump = `
		UPDATE gamification_profiles
		SET points_balance = points_balance + $2, updated_at = NOW()
		WHERE user_id = $1
	`
	if err := r.exec(ctx, bump, userID, amount); err != nil {
		return false, fmt.Errorf("bump points: %w", err)
	}
	return true, nil
}

func (r *Repository) Purchase(ctx context.Context, userID, itemID uuid.UUID) error {
	item, err := r.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	if !item.IsActive {
		return contract.ErrNotFound
	}
	if err := r.EnsureProfile(ctx, userID); err != nil {
		return err
	}
	const pay = `
		UPDATE gamification_profiles
		SET points_balance = points_balance - $2, updated_at = NOW()
		WHERE user_id = $1 AND points_balance >= $2
	`
	tag, err := r.execTag(ctx, pay, userID, item.Price)
	if err != nil {
		return fmt.Errorf("pay points: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrInsufficient
	}
	const buy = `
		INSERT INTO shop_purchases (user_id, item_id)
		VALUES ($1, $2)
	`
	if err := r.exec(ctx, buy, userID, itemID); err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) Equip(ctx context.Context, userID uuid.UUID, background, avatarFrame, backdrop *string) (ports.Profile, error) {
	if err := r.EnsureProfile(ctx, userID); err != nil {
		return ports.Profile{}, err
	}
	if err := r.checkOwned(ctx, userID, background, domain.KindBackground); err != nil {
		return ports.Profile{}, err
	}
	if err := r.checkOwned(ctx, userID, avatarFrame, domain.KindAvatarFrame); err != nil {
		return ports.Profile{}, err
	}
	if err := r.checkOwned(ctx, userID, backdrop, domain.KindBackdrop); err != nil {
		return ports.Profile{}, err
	}
	const q = `
		UPDATE gamification_profiles
		SET equipped_background = CASE WHEN $2::boolean THEN $3 ELSE equipped_background END,
			equipped_avatar_frame = CASE WHEN $4::boolean THEN $5 ELSE equipped_avatar_frame END,
			equipped_backdrop = CASE WHEN $6::boolean THEN $7 ELSE equipped_backdrop END,
			updated_at = NOW()
		WHERE user_id = $1
		RETURNING user_id, points_balance,
			COALESCE(equipped_background, ''),
			COALESCE(equipped_avatar_frame, ''),
			COALESCE(equipped_backdrop, '')
	`
	setBG, bg := ptrSQL(background)
	setAF, af := ptrSQL(avatarFrame)
	setBD, bd := ptrSQL(backdrop)
	var p ports.Profile
	err := r.queryRow(ctx, q, userID, setBG, bg, setAF, af, setBD, bd).Scan(
		&p.UserID, &p.PointsBalance, &p.EquippedBackground, &p.EquippedAvatarFrame, &p.EquippedBackdrop,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Profile{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.Profile{}, mapWriteError(err)
	}
	return p, nil
}

func (r *Repository) checkOwned(ctx context.Context, userID uuid.UUID, code *string, kind string) error {
	if code == nil {
		return nil
	}
	if *code == "" {
		return nil
	}
	if !domain.ValidCode(*code) {
		return contract.FieldError{Field: "code"}
	}
	ok, err := r.OwnsCode(ctx, userID, *code, kind)
	if err != nil {
		return err
	}
	if !ok {
		return contract.ErrNotOwned
	}
	return nil
}

func (r *Repository) CreateAchievement(ctx context.Context, item domain.Achievement) error {
	const q = `
		INSERT INTO achievements (id, code, title, description, metric, target, reward_points, is_active, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if err := r.exec(ctx, q,
		item.ID, item.Code, item.Title, item.Description, item.Metric,
		item.Target, item.RewardPoints, item.IsActive, item.SortOrder,
	); err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) UpdateAchievement(
	ctx context.Context, id uuid.UUID,
	title, description *string, target, reward, sortOrder *int, active *bool,
) (ports.AchievementItem, error) {
	const q = `
		UPDATE achievements
		SET title = COALESCE($2, title),
			description = COALESCE($3, description),
			target = COALESCE($4, target),
			reward_points = COALESCE($5, reward_points),
			sort_order = COALESCE($6, sort_order),
			is_active = COALESCE($7, is_active),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, code, title, description, metric, target, reward_points, is_active, sort_order
	`
	var item ports.AchievementItem
	err := r.queryRow(ctx, q, id, title, description, target, reward, sortOrder, active).Scan(
		&item.ID, &item.Code, &item.Title, &item.Description, &item.Metric,
		&item.Target, &item.RewardPoints, &item.IsActive, &item.SortOrder,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.AchievementItem{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.AchievementItem{}, mapWriteError(err)
	}
	return item, nil
}

func (r *Repository) NextAchievementSort(ctx context.Context) (int, error) {
	const q = `SELECT COALESCE(MAX(sort_order), 0) + 10 FROM achievements`
	var next int
	if err := r.queryRow(ctx, q).Scan(&next); err != nil {
		return 0, fmt.Errorf("next achievement sort: %w", err)
	}
	if next > maxSort {
		return 0, contract.FieldError{Field: "sortOrder"}
	}
	return next, nil
}

func (r *Repository) RecordMetricEvent(ctx context.Context, userID uuid.UUID, metric string, delta int, sourceKey string) (bool, error) {
	if userID == uuid.Nil || metric == "" || delta < 1 || sourceKey == "" {
		return false, contract.ErrInvalid
	}
	const q = `
		INSERT INTO achievement_metric_events (source_key, user_id, metric, delta)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_key) DO NOTHING
	`
	tag, err := r.execTag(ctx, q, sourceKey, userID, metric, delta)
	if err != nil {
		return false, mapWriteError(err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) BumpMetric(ctx context.Context, userID uuid.UUID, metric string, delta int) (int, error) {
	const q = `
		INSERT INTO user_achievement_metrics (user_id, metric, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, metric) DO UPDATE
		SET value = user_achievement_metrics.value + EXCLUDED.value,
			updated_at = NOW()
		RETURNING value
	`
	var value int
	if err := r.queryRow(ctx, q, userID, metric, delta).Scan(&value); err != nil {
		return 0, fmt.Errorf("bump achievement metric: %w", err)
	}
	return value, nil
}

func (r *Repository) ListUnlockable(ctx context.Context, userID uuid.UUID, metric string, value int) ([]ports.UnlockCandidate, error) {
	const q = `
		SELECT a.id, a.code, a.reward_points
		FROM achievements a
		WHERE a.is_active = TRUE
		  AND a.metric = $2
		  AND a.target <= $3
		  AND NOT EXISTS (
			SELECT 1 FROM user_achievements ua
			WHERE ua.user_id = $1 AND ua.achievement_id = a.id
		  )
		ORDER BY a.target ASC, a.sort_order ASC
	`
	rows, err := r.query(ctx, q, userID, metric, value)
	if err != nil {
		return nil, fmt.Errorf("list unlockable: %w", err)
	}
	defer rows.Close()
	out := make([]ports.UnlockCandidate, 0)
	for rows.Next() {
		var c ports.UnlockCandidate
		if err := rows.Scan(&c.ID, &c.Code, &c.RewardPoints); err != nil {
			return nil, fmt.Errorf("scan unlockable: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) UnlockAchievement(ctx context.Context, userID, achievementID uuid.UUID) (bool, error) {
	const q = `
		INSERT INTO user_achievements (user_id, achievement_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, achievement_id) DO NOTHING
	`
	tag, err := r.execTag(ctx, q, userID, achievementID)
	if err != nil {
		return false, mapWriteError(err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) OwnsCode(ctx context.Context, userID uuid.UUID, code, kind string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM shop_purchases p
			INNER JOIN shop_items i ON i.id = p.item_id
			WHERE p.user_id = $1 AND i.code = $2 AND i.kind = $3
		)
	`
	var ok bool
	if err := r.queryRow(ctx, q, userID, code, kind).Scan(&ok); err != nil {
		return false, fmt.Errorf("owns code: %w", err)
	}
	return ok, nil
}

func ptrSQL(v *string) (bool, any) {
	if v == nil {
		return false, nil
	}
	if *v == "" {
		return true, nil
	}
	return true, *v
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.execTag(ctx, sql, args...)
	return err
}

func (r *Repository) execTag(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	if r == nil || r.pool == nil {
		return pgconn.CommandTag{}, contract.ErrInvalid
	}
	return r.pool.Exec(ctx, sql, args...)
}

func (r *Repository) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}

func (r *Repository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	return r.pool.Query(ctx, sql, args...)
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return contract.ErrConflict
		case "23514":
			return contract.FieldError{Field: "body"}
		case "23503":
			return contract.ErrNotFound
		}
	}
	return err
}
