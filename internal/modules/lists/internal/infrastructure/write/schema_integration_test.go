package write_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"review-api/internal/platform/database"
)

// TestListsSchemaIntegration checks the invariants the migration enforces on its own,
// independent of application code.
func TestListsSchemaIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := testPool(t, ctx)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	wantCode := func(code, sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Fatalf("%s: err = %v, want SQLSTATE %s", sql, err, code)
		}
	}
	const (
		fkViolation     = "23503"
		uniqueViolation = "23505"
		checkViolation  = "23514"
	)

	user, other := uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, telegram_id, first_name) VALUES ($1, 1, 'Ann'), ($2, 2, 'Bob')`, user, other)
	work := uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES ($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg')`, work)

	tplA, tplB := uuid.New(), uuid.New()
	exec(`INSERT INTO list_templates (id, author_id, type_code, title) VALUES ($1, $3, 'game', 'A'), ($2, $3, 'game', 'B')`, tplA, tplB, user)

	// A mixed template has no type; "mixed" stays reserved and never enters the review dictionary.
	mixedTpl := uuid.New()
	exec(`INSERT INTO list_templates (id, author_id, type_code) VALUES ($1, $2, NULL)`, mixedTpl, user)
	wantCode(checkViolation, `INSERT INTO review_types (code, label, sort_order) VALUES ('mixed', 'Смесь', 1)`)

	// Template items: a catalog work at most once; the catalog row may disappear later.
	itemWork := uuid.New()
	exec(`INSERT INTO list_template_items (template_id, id, position, title, media_object_id, image_url) VALUES ($1, $2, 0, 'Halo', $3, 'https://cdn.example/halo.jpg')`, tplA, itemWork, work)
	wantCode(uniqueViolation, `INSERT INTO list_template_items (template_id, id, position, title, media_object_id) VALUES ($1, $2, 1, 'Halo again', $3)`, tplA, uuid.New(), work)
	exec(`INSERT INTO list_template_items (template_id, id, position, title) VALUES ($1, $2, 1, 'Same'), ($1, $3, 2, 'Same')`, tplA, uuid.New(), uuid.New())
	wantCode(checkViolation, `INSERT INTO list_template_items (template_id, id, position, image_id) VALUES ($1, $2, 3, $2)`, tplA, uuid.New())
	wantCode(checkViolation, `INSERT INTO list_template_tiers (template_id, id, position, name, color) VALUES ($1, $2, 0, 'S', 'red')`, tplA, uuid.New())
	wantCode(checkViolation, `INSERT INTO list_template_tiers (template_id, id, position, name, color) VALUES ($1, $2, 10, 'S', '#ffffff')`, tplA, uuid.New())

	// A published row needs a first publication date and a title.
	wantCode(checkViolation, `UPDATE list_templates SET status = 'published' WHERE id = $1`, tplA)
	wantCode(checkViolation, `UPDATE list_templates SET status = 'published', first_published_at = NOW(), title = '' WHERE id = $1`, tplA)
	exec(`UPDATE list_templates SET status = 'published', first_published_at = NOW() WHERE id = $1`, tplA)

	// Snapshots for both templates.
	verA, verB := uuid.New(), uuid.New()
	exec(`INSERT INTO list_template_versions (id, template_id, revision, author_id, type_code, format, title) VALUES ($1, $2, 1, $5, 'game', 'tiers', 'A'), ($3, $4, 1, $5, 'game', 'tiers', 'B')`, verA, tplA, verB, tplB, user)
	itemA1, itemA2, itemB := uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO list_template_version_items (version_id, item_id, position, title, media_object_id, image_url) VALUES
		($1, $2, 0, 'Halo', $6, 'https://cdn.example/halo.jpg'), ($1, $3, 1, 'Two', NULL, '/storage/images/x.png'), ($4, $5, 0, 'B', NULL, '/storage/images/y.png')`,
		verA, itemA1, itemA2, verB, itemB, work)
	tierS, tierA, tierB := uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO list_template_version_tiers (version_id, tier_id, position, name, color) VALUES ($1, $2, 0, 'S', '#ff7f7f'), ($1, $3, 1, 'A', '#ffbf7f'), ($4, $5, 0, 'S', '#ff7f7f')`, verA, tierS, tierA, verB, tierB)

	// A result must use a version of its own template.
	wantCode(fkViolation, `INSERT INTO list_results (id, author_id, template_id, version_id) VALUES ($1, $2, $3, $4)`, uuid.New(), other, tplA, verB)
	res := uuid.New()
	exec(`INSERT INTO list_results (id, author_id, template_id, version_id, title) VALUES ($1, $2, $3, $4, 'Мой топ')`, res, other, tplA, verA)

	// Entries: only items and tiers of the result's snapshot, unique places per tier.
	exec(`INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, $4, 1)`, res, verA, itemA1, tierS)
	wantCode(fkViolation, `INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, $4, 1)`, res, verA, itemB, tierA)
	wantCode(fkViolation, `INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, $4, 1)`, res, verA, itemA2, tierB)
	wantCode(fkViolation, `INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, NULL, 1)`, res, verB, itemB)
	wantCode(uniqueViolation, `INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, $4, 1)`, res, verA, itemA2, tierS)
	wantCode(uniqueViolation, `INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, $4, 2)`, res, verA, itemA1, tierA)
	exec(`INSERT INTO list_result_entries (result_id, version_id, item_id, tier_id, position) VALUES ($1, $2, $3, $4, 1)`, res, verA, itemA2, tierA)

	// Places without tiers (ranked/collection) are unique too: NULL tier is not distinct.
	ranked := uuid.New()
	exec(`INSERT INTO list_results (id, author_id, template_id, version_id) VALUES ($1, $2, $3, $4)`, ranked, other, tplA, verA)
	exec(`INSERT INTO list_result_entries (result_id, version_id, item_id, position) VALUES ($1, $2, $3, 1)`, ranked, verA, itemA1)
	wantCode(uniqueViolation, `INSERT INTO list_result_entries (result_id, version_id, item_id, position) VALUES ($1, $2, $3, 1)`, ranked, verA, itemA2)

	// Removing a work from the catalog keeps title and image, only the link disappears.
	exec(`DELETE FROM media_objects WHERE id = $1`, work)
	var (
		link  *uuid.UUID
		title string
		image string
	)
	if err := pool.QueryRow(ctx, `SELECT media_object_id, title, image_url FROM list_template_version_items WHERE version_id = $1 AND item_id = $2`, verA, itemA1).Scan(&link, &title, &image); err != nil {
		t.Fatal(err)
	}
	if link != nil || title != "Halo" || image != "https://cdn.example/halo.jpg" {
		t.Fatalf("snapshot after catalog delete: %v %q %q", link, title, image)
	}
	if err := pool.QueryRow(ctx, `SELECT media_object_id, title FROM list_template_items WHERE template_id = $1 AND id = $2`, tplA, itemWork).Scan(&link, &title); err != nil {
		t.Fatal(err)
	}
	if link != nil || title != "Halo" {
		t.Fatalf("template item after catalog delete: %v %q", link, title)
	}

	// Reports: exactly one target, one open report per reporter and object.
	wantCode(checkViolation, `INSERT INTO list_reports (id, template_id, result_id, reporter_id, reason_code) VALUES ($1, $2, $3, $4, 'spam')`, uuid.New(), tplA, res, user)
	wantCode(checkViolation, `INSERT INTO list_reports (id, reporter_id, reason_code) VALUES ($1, $2, 'spam')`, uuid.New(), user)
	wantCode(fkViolation, `INSERT INTO list_reports (id, result_id, reporter_id, reason_code) VALUES ($1, $2, $3, 'not_a_reason')`, uuid.New(), res, user)
	first := uuid.New()
	exec(`INSERT INTO list_reports (id, result_id, reporter_id, reason_code) VALUES ($1, $2, $3, 'spam')`, first, res, user)
	wantCode(uniqueViolation, `INSERT INTO list_reports (id, result_id, reporter_id, reason_code) VALUES ($1, $2, $3, 'abuse')`, uuid.New(), res, user)
	exec(`INSERT INTO list_reports (id, template_id, reporter_id, reason_code) VALUES ($1, $2, $3, 'spam')`, uuid.New(), tplA, user)
	wantCode(checkViolation, `UPDATE list_reports SET status = 'resolved' WHERE id = $1`, first)
	exec(`UPDATE list_reports SET status = 'resolved', resolved_at = NOW() WHERE id = $1`, first)
	exec(`INSERT INTO list_reports (id, result_id, reporter_id, reason_code) VALUES ($1, $2, $3, 'abuse')`, uuid.New(), res, user)

	// Rolling back mixed templates is refused while any exist: older code cannot read a NULL type.
	if err := database.MigrateDown(ctx, pool); err == nil || !strings.Contains(err.Error(), "mixed list templates exist") {
		t.Fatalf("down with a mixed template: %v", err)
	}
	exec(`DELETE FROM list_templates WHERE id = $1`, mixedTpl)

	// The down migration removes the module cleanly and up re-applies it.
	if err := database.MigrateDown(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
