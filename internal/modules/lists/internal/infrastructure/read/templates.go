package read

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Template loads a template that is not deleted, with its author. Access rules are applied by the caller.
func (r *Repository) Template(ctx context.Context, id uuid.UUID) (ports.TemplateView, error) {
	if r == nil || r.pool == nil || id == uuid.Nil {
		return ports.TemplateView{}, contract.ErrNotFound
	}
	const q = `
		SELECT t.id, t.author_id, COALESCE(t.type_code, 'mixed'), t.format, t.title, t.description, t.status, t.visibility,
			t.revision, t.first_published_at, t.moderation_hidden_at, t.created_at, t.updated_at,
			COALESCE(u.username, ''), COALESCE(u.photo_url, '')
		FROM list_templates t
		JOIN users u ON u.id = t.author_id
		WHERE t.id = $1 AND t.deleted_at IS NULL`
	var v ports.TemplateView
	t := &v.Template
	err := r.pool.QueryRow(ctx, q, id).Scan(&t.ID, &t.AuthorID, &t.TypeCode, &t.Format, &t.Title, &t.Description,
		&t.Status, &t.Visibility, &t.Revision, &t.FirstPublishedAt, &t.ModerationHiddenAt, &t.CreatedAt, &v.UpdatedAt,
		&v.Author.Username, &v.Author.PhotoURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.TemplateView{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.TemplateView{}, fmt.Errorf("read list template: %w", err)
	}
	v.Author.ID = t.AuthorID

	rows, err := r.pool.Query(ctx, `SELECT id, name, color FROM list_template_tiers WHERE template_id = $1 ORDER BY position`, id)
	if err != nil {
		return ports.TemplateView{}, fmt.Errorf("read list template tiers: %w", err)
	}
	t.Tiers, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Tier, error) {
		var tier domain.Tier
		err := row.Scan(&tier.ID, &tier.Name, &tier.Color)
		return tier, err
	})
	if err != nil {
		return ports.TemplateView{}, fmt.Errorf("read list template tiers: %w", err)
	}
	rows, err = r.pool.Query(ctx, `
		SELECT i.id, i.title, i.media_object_id, i.image_id, COALESCE(i.image_url, ''), COALESCE(mo.type::text, '')
		FROM list_template_items i
		LEFT JOIN media_objects mo ON mo.id = i.media_object_id
		WHERE i.template_id = $1 ORDER BY i.position`, id)
	if err != nil {
		return ports.TemplateView{}, fmt.Errorf("read list template items: %w", err)
	}
	t.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) {
		var it domain.Item
		err := row.Scan(&it.ID, &it.Title, &it.MediaObjectID, &it.ImageID, &it.ImageURL, &it.MediaType)
		return it, err
	})
	if err != nil {
		return ports.TemplateView{}, fmt.Errorf("read list template items: %w", err)
	}
	return v, nil
}
