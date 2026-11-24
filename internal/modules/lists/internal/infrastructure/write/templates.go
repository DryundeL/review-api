package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/platform/transaction"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// LockAuthor takes a transaction-scoped advisory lock keyed by the author.
func (r *Repository) LockAuthor(ctx context.Context, authorID uuid.UUID) error {
	tx, ok := transaction.TxFromContext(ctx)
	if !ok {
		return contract.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('lists:author:' || $1::text, 0))`, authorID); err != nil {
		return fmt.Errorf("lock list author: %w", err)
	}
	return nil
}

func (r *Repository) CountTemplates(ctx context.Context, authorID uuid.UUID) (int, error) {
	var n int
	err := r.queryRow(ctx, `SELECT COUNT(*)::int FROM list_templates WHERE author_id = $1 AND deleted_at IS NULL`, authorID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count list templates: %w", err)
	}
	return n, nil
}

func (r *Repository) InsertTemplate(ctx context.Context, t domain.Template) error {
	const q = `
		INSERT INTO list_templates (id, author_id, type_code, format, title, description, status, visibility,
			revision, first_published_at, moderation_hidden_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)`
	if _, err := r.exec(ctx, q, t.ID, t.AuthorID, t.TypeCode, t.Format, t.Title, t.Description, t.Status,
		t.Visibility, t.Revision, t.FirstPublishedAt, t.ModerationHiddenAt, t.CreatedAt); err != nil {
		return mapTemplateError(err)
	}
	return r.insertContent(ctx, t)
}

func (r *Repository) GetTemplateForUpdate(ctx context.Context, id uuid.UUID) (domain.Template, error) {
	if id == uuid.Nil {
		return domain.Template{}, contract.ErrNotFound
	}
	const q = `
		SELECT id, author_id, type_code, format, title, description, status, visibility, revision,
			first_published_at, moderation_hidden_at, created_at
		FROM list_templates
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`
	var t domain.Template
	err := r.queryRow(ctx, q, id).Scan(&t.ID, &t.AuthorID, &t.TypeCode, &t.Format, &t.Title, &t.Description,
		&t.Status, &t.Visibility, &t.Revision, &t.FirstPublishedAt, &t.ModerationHiddenAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Template{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Template{}, fmt.Errorf("get list template: %w", err)
	}
	if t.Tiers, err = r.tiers(ctx, id); err != nil {
		return domain.Template{}, err
	}
	if t.Items, err = r.items(ctx, id); err != nil {
		return domain.Template{}, err
	}
	return t, nil
}

func (r *Repository) tiers(ctx context.Context, templateID uuid.UUID) ([]domain.Tier, error) {
	rows, err := r.query(ctx, `SELECT id, name, color FROM list_template_tiers WHERE template_id = $1 ORDER BY position`, templateID)
	if err != nil {
		return nil, fmt.Errorf("list template tiers: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Tier, error) {
		var t domain.Tier
		err := row.Scan(&t.ID, &t.Name, &t.Color)
		return t, err
	})
}

func (r *Repository) items(ctx context.Context, templateID uuid.UUID) ([]domain.Item, error) {
	rows, err := r.query(ctx, `
		SELECT id, title, media_object_id, image_id, COALESCE(image_url, '')
		FROM list_template_items WHERE template_id = $1 ORDER BY position`, templateID)
	if err != nil {
		return nil, fmt.Errorf("list template items: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) {
		var it domain.Item
		err := row.Scan(&it.ID, &it.Title, &it.MediaObjectID, &it.ImageID, &it.ImageURL)
		return it, err
	})
}

// UpdateTemplate stores the header; withContent also replaces tiers and items.
func (r *Repository) UpdateTemplate(ctx context.Context, t domain.Template, withContent bool) error {
	const q = `
		UPDATE list_templates
		SET type_code = $2, format = $3, title = $4, description = $5, status = $6, visibility = $7,
			revision = $8, first_published_at = $9, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.exec(ctx, q, t.ID, t.TypeCode, t.Format, t.Title, t.Description, t.Status, t.Visibility,
		t.Revision, t.FirstPublishedAt)
	if err != nil {
		return mapTemplateError(err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	if !withContent {
		return nil
	}
	if _, err := r.exec(ctx, `DELETE FROM list_template_items WHERE template_id = $1`, t.ID); err != nil {
		return mapTemplateError(err)
	}
	if _, err := r.exec(ctx, `DELETE FROM list_template_tiers WHERE template_id = $1`, t.ID); err != nil {
		return mapTemplateError(err)
	}
	return r.insertContent(ctx, t)
}

// insertContent writes tiers and items in their slice order: position is the array index.
func (r *Repository) insertContent(ctx context.Context, t domain.Template) error {
	if len(t.Tiers) > 0 {
		ids, names, colors := make([]uuid.UUID, len(t.Tiers)), make([]string, len(t.Tiers)), make([]string, len(t.Tiers))
		for i, tier := range t.Tiers {
			ids[i], names[i], colors[i] = tier.ID, tier.Name, tier.Color
		}
		const q = `
			INSERT INTO list_template_tiers (template_id, id, position, name, color)
			SELECT $1, x.id, x.ord - 1, x.name, x.color
			FROM unnest($2::uuid[], $3::text[], $4::text[]) WITH ORDINALITY AS x(id, name, color, ord)`
		if _, err := r.exec(ctx, q, t.ID, ids, names, colors); err != nil {
			return mapTemplateError(err)
		}
	}
	if len(t.Items) == 0 {
		return nil
	}
	n := len(t.Items)
	ids, titles := make([]uuid.UUID, n), make([]string, n)
	works, images, urls := make([]*uuid.UUID, n), make([]*uuid.UUID, n), make([]*string, n)
	for i, it := range t.Items {
		ids[i], titles[i], works[i], images[i] = it.ID, it.Title, it.MediaObjectID, it.ImageID
		if it.ImageURL != "" {
			url := it.ImageURL
			urls[i] = &url
		}
	}
	const q = `
		INSERT INTO list_template_items (template_id, id, position, title, media_object_id, image_id, image_url)
		SELECT $1, x.id, x.ord - 1, x.title, x.work, x.image, x.url
		FROM unnest($2::uuid[], $3::text[], $4::uuid[], $5::uuid[], $6::text[]) WITH ORDINALITY AS x(id, title, work, image, url, ord)`
	if _, err := r.exec(ctx, q, t.ID, ids, titles, works, images, urls); err != nil {
		return mapTemplateError(err)
	}
	return nil
}

func (r *Repository) SoftDeleteTemplate(ctx context.Context, id uuid.UUID) error {
	tag, err := r.exec(ctx, `UPDATE list_templates SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapTemplateError(err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
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

// mapTemplateError turns constraint violations that can follow a race (a work deleted from the
// catalog, a type removed) into field errors. Everything else the domain has already rejected.
func mapTemplateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		switch pgErr.ConstraintName {
		case "list_template_items_media_object_id_fkey":
			return contract.Field("items.mediaObjectId", contract.ReasonInvalid)
		case "list_template_items_image_id_fkey":
			return contract.Field("items.imageId", contract.ReasonInvalid)
		case "list_templates_type_code_fkey":
			return contract.Field("typeCode", contract.ReasonInvalid)
		}
		return fmt.Errorf("%w: %s", contract.ErrInvalid, pgErr.ConstraintName)
	case "23505":
		if pgErr.ConstraintName == "list_template_items_work_uidx" {
			return contract.Field("items.mediaObjectId", contract.ReasonDuplicate)
		}
		return contract.ErrConflict
	case "23514":
		return fmt.Errorf("%w: %s", contract.ErrInvalid, pgErr.ConstraintName)
	}
	return err
}
