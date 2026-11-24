package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/domain"
)

func (r *Repository) CountResults(ctx context.Context, authorID uuid.UUID) (int, error) {
	var n int
	err := r.queryRow(ctx, `SELECT COUNT(*)::int FROM list_results WHERE author_id = $1 AND deleted_at IS NULL`, authorID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count list results: %w", err)
	}
	return n, nil
}

// TemplateModerated reads the moderation flag of a template, deleted ones included, under FOR SHARE.
func (r *Repository) TemplateModerated(ctx context.Context, templateID uuid.UUID) (bool, error) {
	var hidden bool
	err := r.queryRow(ctx, `SELECT moderation_hidden_at IS NOT NULL FROM list_templates WHERE id = $1 FOR SHARE`, templateID).Scan(&hidden)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, contract.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("read list template moderation: %w", err)
	}
	return hidden, nil
}

// EnsureSnapshot copies the working state of the locked template into an immutable version,
// once per revision. A concurrent insert of the same revision waits on the unique key and
// then reuses the committed version.
func (r *Repository) EnsureSnapshot(ctx context.Context, t domain.Template) (domain.Snapshot, error) {
	const insertVersion = `
		INSERT INTO list_template_versions (id, template_id, revision, author_id, type_code, format, title)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT ON CONSTRAINT list_template_versions_revision_key DO NOTHING`
	versionID := uuid.New()
	tag, err := r.exec(ctx, insertVersion, versionID, t.ID, t.Revision, t.AuthorID, t.TypeCode, t.Format, t.Title)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("insert list template version: %w", err)
	}
	if tag.RowsAffected() == 1 {
		const copyTiers = `
			INSERT INTO list_template_version_tiers (version_id, tier_id, position, name, color)
			SELECT $1, id, position, name, color FROM list_template_tiers WHERE template_id = $2`
		if _, err := r.exec(ctx, copyTiers, versionID, t.ID); err != nil {
			return domain.Snapshot{}, fmt.Errorf("copy list template tiers: %w", err)
		}
		const copyItems = `
			INSERT INTO list_template_version_items (version_id, item_id, position, title, media_object_id, image_url)
			SELECT $1, id, position, title, media_object_id, image_url FROM list_template_items WHERE template_id = $2`
		if _, err := r.exec(ctx, copyItems, versionID, t.ID); err != nil {
			return domain.Snapshot{}, fmt.Errorf("copy list template items: %w", err)
		}
	} else {
		err := r.queryRow(ctx, `SELECT id FROM list_template_versions WHERE template_id = $1 AND revision = $2`,
			t.ID, t.Revision).Scan(&versionID)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("read list template version: %w", err)
		}
	}
	return r.Snapshot(ctx, versionID)
}

func (r *Repository) Snapshot(ctx context.Context, versionID uuid.UUID) (domain.Snapshot, error) {
	s := domain.Snapshot{VersionID: versionID}
	err := r.queryRow(ctx, `SELECT template_id, format, title FROM list_template_versions WHERE id = $1`, versionID).
		Scan(&s.TemplateID, &s.Format, &s.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Snapshot{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("read list template version: %w", err)
	}
	rows, err := r.query(ctx, `
		SELECT tier_id, name, color FROM list_template_version_tiers WHERE version_id = $1 ORDER BY position`, versionID)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("read list version tiers: %w", err)
	}
	if s.Tiers, err = pgx.CollectRows(rows, scanTier); err != nil {
		return domain.Snapshot{}, fmt.Errorf("read list version tiers: %w", err)
	}
	rows, err = r.query(ctx, `
		SELECT item_id, title, media_object_id, image_url FROM list_template_version_items
		WHERE version_id = $1 ORDER BY position`, versionID)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("read list version items: %w", err)
	}
	s.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) {
		var it domain.Item
		err := row.Scan(&it.ID, &it.Title, &it.MediaObjectID, &it.ImageURL)
		return it, err
	})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("read list version items: %w", err)
	}
	return s, nil
}

func scanTier(row pgx.CollectableRow) (domain.Tier, error) {
	var t domain.Tier
	err := row.Scan(&t.ID, &t.Name, &t.Color)
	return t, err
}

func (r *Repository) InsertResult(ctx context.Context, res domain.Result) error {
	const q = `
		INSERT INTO list_results (id, author_id, template_id, version_id, title, description, status, visibility,
			revision, first_published_at, moderation_hidden_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)`
	if _, err := r.exec(ctx, q, res.ID, res.AuthorID, res.TemplateID, res.VersionID, res.Title, res.Description,
		res.Status, res.Visibility, res.Revision, res.FirstPublishedAt, res.ModerationHiddenAt, res.CreatedAt); err != nil {
		return mapResultError(err)
	}
	return r.insertEntries(ctx, res)
}

// GetResultForUpdate locks the result row; the format comes from its snapshot.
func (r *Repository) GetResultForUpdate(ctx context.Context, id uuid.UUID) (domain.Result, error) {
	if id == uuid.Nil {
		return domain.Result{}, contract.ErrNotFound
	}
	const q = `
		SELECT r.id, r.author_id, r.template_id, r.version_id, v.format, r.title, r.description, r.status,
			r.visibility, r.revision, r.first_published_at, r.moderation_hidden_at, r.created_at
		FROM list_results r
		JOIN list_template_versions v ON v.id = r.version_id
		WHERE r.id = $1 AND r.deleted_at IS NULL
		FOR UPDATE OF r`
	var res domain.Result
	err := r.queryRow(ctx, q, id).Scan(&res.ID, &res.AuthorID, &res.TemplateID, &res.VersionID, &res.Format,
		&res.Title, &res.Description, &res.Status, &res.Visibility, &res.Revision, &res.FirstPublishedAt,
		&res.ModerationHiddenAt, &res.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Result{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Result{}, fmt.Errorf("get list result: %w", err)
	}
	// Canonical order: tier order of the snapshot (no tier first), then position.
	rows, err := r.query(ctx, `
		SELECT e.item_id, e.tier_id, e.position
		FROM list_result_entries e
		LEFT JOIN list_template_version_tiers t ON t.version_id = e.version_id AND t.tier_id = e.tier_id
		WHERE e.result_id = $1
		ORDER BY t.position NULLS FIRST, e.position`, id)
	if err != nil {
		return domain.Result{}, fmt.Errorf("list result entries: %w", err)
	}
	res.Entries, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Entry, error) {
		var e domain.Entry
		err := row.Scan(&e.ItemID, &e.TierID, &e.Position)
		return e, err
	})
	if err != nil {
		return domain.Result{}, fmt.Errorf("list result entries: %w", err)
	}
	return res, nil
}

// UpdateResult stores the author's fields; moderation fields are never written here.
func (r *Repository) UpdateResult(ctx context.Context, res domain.Result, withEntries bool) error {
	const q = `
		UPDATE list_results
		SET title = $2, description = $3, status = $4, visibility = $5, revision = $6,
			first_published_at = $7, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.exec(ctx, q, res.ID, res.Title, res.Description, res.Status, res.Visibility, res.Revision,
		res.FirstPublishedAt)
	if err != nil {
		return mapResultError(err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	if !withEntries {
		return nil
	}
	if _, err := r.exec(ctx, `DELETE FROM list_result_entries WHERE result_id = $1`, res.ID); err != nil {
		return mapResultError(err)
	}
	return r.insertEntries(ctx, res)
}

func (r *Repository) insertEntries(ctx context.Context, res domain.Result) error {
	n := len(res.Entries)
	if n == 0 {
		return nil
	}
	items, tiers, positions := make([]uuid.UUID, n), make([]*uuid.UUID, n), make([]int32, n)
	for i, e := range res.Entries {
		items[i], tiers[i], positions[i] = e.ItemID, e.TierID, int32(e.Position)
	}
	const q = `
		INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position)
		SELECT $1, $2, x.item, x.tier, x.position
		FROM unnest($3::uuid[], $4::uuid[], $5::int[]) AS x(item, tier, position)`
	if _, err := r.exec(ctx, q, res.ID, res.VersionID, items, tiers, positions); err != nil {
		return mapResultError(err)
	}
	return nil
}

func (r *Repository) SoftDeleteResult(ctx context.Context, id uuid.UUID) error {
	tag, err := r.exec(ctx, `UPDATE list_results SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapResultError(err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

// mapResultError is the last line of defence: the domain validates entries against the
// snapshot, and the composite foreign keys reject anything that slipped past it.
func mapResultError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		switch pgErr.ConstraintName {
		case "list_result_entries_item_fkey":
			return contract.Field("entries.itemId", contract.ReasonInvalid)
		case "list_result_entries_tier_fkey":
			return contract.Field("entries.tierId", contract.ReasonInvalid)
		}
		return fmt.Errorf("%w: %s", contract.ErrInvalid, pgErr.ConstraintName)
	case "23505":
		switch pgErr.ConstraintName {
		case "list_result_entries_pkey":
			return contract.Field("entries.itemId", contract.ReasonDuplicate)
		case "list_result_entries_position_key":
			return contract.Field("entries.position", contract.ReasonInvalid)
		}
		return contract.ErrConflict
	case "23514":
		return fmt.Errorf("%w: %s", contract.ErrInvalid, pgErr.ConstraintName)
	}
	return err
}
