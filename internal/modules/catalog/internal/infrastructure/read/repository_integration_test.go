package read_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"review-api/internal/modules/catalog/internal/domain"
	catalogread "review-api/internal/modules/catalog/internal/infrastructure/read"
	catalogwrite "review-api/internal/modules/catalog/internal/infrastructure/write"
)

func TestInfoRoundTripIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "test_catalog_" + uuid.New().String()[:8]
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
	_, err = pool.Exec(ctx, `CREATE TABLE media_objects (
 id UUID PRIMARY KEY,title TEXT NOT NULL,original_title TEXT,type TEXT NOT NULL,info JSON NOT NULL,
 poster_url TEXT,external_source TEXT,external_id TEXT,created_by UUID,created_at TIMESTAMPTZ DEFAULT now())`)
	if err != nil {
		t.Fatal(err)
	}
	year := 1999
	item, err := domain.NewImported(domain.Draft{Title: "Matrix", Type: "movie", Info: &domain.Info{Genre: "sci-fi", ReleaseYear: &year, ReleaseStatus: "completed"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogwrite.NewRepository(pool).Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	repo := catalogread.NewRepository(pool)
	got, err := repo.Get(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Info.Genre != "sci-fi" || got.Info.ReleaseYear == nil || *got.Info.ReleaseYear != 1999 || got.Info.ReleaseStatus != "completed" {
		t.Fatalf("info=%+v", got.Info)
	}
	items, err := repo.Search(ctx, "Matrix", "movie", 20)
	if err != nil || len(items) != 1 || items[0].Info.ReleaseStatus != "completed" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}
