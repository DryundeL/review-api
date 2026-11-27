package read

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

// Result loads a result that is not deleted, with its snapshot and the public state of its
// template. The template title and author come from the snapshot, never from a draft.
func (r *Repository) Result(ctx context.Context, id uuid.UUID) (ports.ResultView, error) {
	if r == nil || r.pool == nil || id == uuid.Nil {
		return ports.ResultView{}, contract.ErrNotFound
	}
	const q = `
		SELECT r.id, r.author_id, r.template_id, r.version_id, v.format, r.title, r.description, r.status,
			r.visibility, r.revision, r.first_published_at, r.moderation_hidden_at, r.created_at, r.updated_at,
			COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			v.type_code, v.title, v.author_id, COALESCE(tu.username, ''), COALESCE(tu.photo_url, ''),
			t.moderation_hidden_at IS NOT NULL,
			t.status = 'published' AND t.visibility = 'public' AND t.deleted_at IS NULL AND t.moderation_hidden_at IS NULL,
			r.likes_count, r.dislikes_count, r.views_count
		FROM list_results r
		JOIN list_template_versions v ON v.id = r.version_id
		JOIN list_templates t ON t.id = r.template_id
		JOIN users u ON u.id = r.author_id
		JOIN users tu ON tu.id = v.author_id
		WHERE r.id = $1 AND r.deleted_at IS NULL`
	var v ports.ResultView
	res, snap := &v.Result, &v.Snapshot
	err := r.pool.QueryRow(ctx, q, id).Scan(&res.ID, &res.AuthorID, &res.TemplateID, &res.VersionID, &res.Format,
		&res.Title, &res.Description, &res.Status, &res.Visibility, &res.Revision, &res.FirstPublishedAt,
		&res.ModerationHiddenAt, &res.CreatedAt, &v.UpdatedAt, &v.Author.Username, &v.Author.PhotoURL,
		&v.TypeCode, &snap.Title, &v.TemplateAuthor.ID, &v.TemplateAuthor.Username, &v.TemplateAuthor.PhotoURL,
		&v.TemplateModerated, &v.TemplatePublic, &v.LikesCount, &v.DislikesCount, &v.ViewsCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ResultView{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list result: %w", err)
	}
	v.Author.ID = res.AuthorID
	snap.VersionID, snap.TemplateID, snap.Format = res.VersionID, res.TemplateID, res.Format

	rows, err := r.pool.Query(ctx, `
		SELECT tier_id, name, color FROM list_template_version_tiers WHERE version_id = $1 ORDER BY position`, res.VersionID)
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list version tiers: %w", err)
	}
	snap.Tiers, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Tier, error) {
		var tier domain.Tier
		err := row.Scan(&tier.ID, &tier.Name, &tier.Color)
		return tier, err
	})
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list version tiers: %w", err)
	}
	rows, err = r.pool.Query(ctx, `
		SELECT item_id, title, media_object_id, image_url FROM list_template_version_items
		WHERE version_id = $1 ORDER BY position`, res.VersionID)
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list version items: %w", err)
	}
	snap.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) {
		var it domain.Item
		err := row.Scan(&it.ID, &it.Title, &it.MediaObjectID, &it.ImageURL)
		return it, err
	})
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list version items: %w", err)
	}
	rows, err = r.pool.Query(ctx, `
		SELECT e.item_id, e.tier_id, e.position
		FROM list_result_entries e
		LEFT JOIN list_template_version_tiers t ON t.version_id = e.version_id AND t.tier_id = e.tier_id
		WHERE e.result_id = $1
		ORDER BY t.position NULLS FIRST, e.position`, id)
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list result entries: %w", err)
	}
	res.Entries, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Entry, error) {
		var e domain.Entry
		err := row.Scan(&e.ItemID, &e.TierID, &e.Position)
		return e, err
	})
	if err != nil {
		return ports.ResultView{}, fmt.Errorf("read list result entries: %w", err)
	}
	return v, nil
}
