package write_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/ports"
	"review-api/internal/modules/media/internal/infrastructure/write"
	"review-api/internal/platform/database"
	"review-api/internal/platform/transaction"
)

func TestInsertImageIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "test_media_" + uuid.New().String()[:8]
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}

	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, telegram_id, first_name) VALUES ($1, 1, 'Ann')`, owner); err != nil {
		t.Fatal(err)
	}
	repo := write.NewRepository(pool)
	valid := func() ports.ImageRecord {
		id := uuid.New()
		return ports.ImageRecord{
			ID: id, OwnerID: owner, StorageKey: "images/" + id.String() + ".png",
			ContentType: "image/png", SizeBytes: 100, Width: 10, Height: 10, CreatedAt: time.Now().UTC(),
		}
	}
	if err := repo.Insert(ctx, valid()); err != nil {
		t.Fatalf("valid insert: %v", err)
	}

	for name, mutate := range map[string]func(*ports.ImageRecord){
		"unknown owner":   func(r *ports.ImageRecord) { r.OwnerID = uuid.New() },
		"traversal key":   func(r *ports.ImageRecord) { r.StorageKey = "images/../x.png" },
		"svg type":        func(r *ports.ImageRecord) { r.ContentType = "image/svg+xml" },
		"over 300 KB":     func(r *ports.ImageRecord) { r.SizeBytes = contract.MaxImageBytes + 1 },
		"too wide":        func(r *ports.ImageRecord) { r.Width = contract.MaxImageSide + 1 },
		"zero dimensions": func(r *ports.ImageRecord) { r.Height = 0 },
	} {
		rec := valid()
		mutate(&rec)
		if err := repo.Insert(ctx, rec); !errors.Is(err, contract.ErrInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}

	// GC: only old unattached rows are claimed; attached rows survive DeleteByIDs.
	if _, err := pool.Exec(ctx, `DELETE FROM media_images`); err != nil {
		t.Fatal(err)
	}
	old, fresh, attached := valid(), valid(), valid()
	old.CreatedAt = time.Now().UTC().Add(-48 * time.Hour)
	attached.CreatedAt = old.CreatedAt
	for _, rec := range []ports.ImageRecord{old, fresh, attached} {
		if err := repo.Insert(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE media_images SET attached_at = NOW() WHERE id = $1`, attached.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LockUnattached(ctx, time.Now(), 10); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("LockUnattached outside TX: %v", err)
	}
	err = transaction.NewPgxManager(pool).WithinTransaction(ctx, func(txCtx context.Context) error {
		items, err := repo.LockUnattached(txCtx, time.Now().UTC().Add(-24*time.Hour), 10)
		if err != nil {
			return err
		}
		if len(items) != 1 || items[0].ID != old.ID || items[0].StorageKey != old.StorageKey {
			t.Fatalf("claimed %+v", items)
		}
		return repo.DeleteByIDs(txCtx, []uuid.UUID{old.ID, attached.ID})
	})
	if err != nil {
		t.Fatal(err)
	}
	var left []uuid.UUID
	rows, _ := pool.Query(ctx, `SELECT id FROM media_images ORDER BY created_at`)
	for rows.Next() {
		var id uuid.UUID
		_ = rows.Scan(&id)
		left = append(left, id)
	}
	if len(left) != 2 || left[0] != attached.ID || left[1] != fresh.ID {
		t.Fatalf("left = %v", left)
	}
}
