package read

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/platform/database"
	"review-api/migrations"
)

func TestTopByMediaIntegration(t *testing.T) {
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
	schema := "test_top_" + uuid.New().String()[:8]
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
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, db, ".", 19); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	mediaID, otherID := uuid.New(), uuid.New()
	exec(`INSERT INTO media_objects (id,title,type,genre,release_year,release_status) VALUES ($1,'Matrix','movie','sci-fi',1999,'completed'),($2,'Other','movie',NULL,NULL,'announced')`, mediaID, otherID)
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var genre, status string
	var year int
	if err := pool.QueryRow(ctx, `SELECT info->>'genre',(info->>'releaseYear')::int,info->>'releaseStatus' FROM media_objects WHERE id=$1`, mediaID).Scan(&genre, &year, &status); err != nil {
		t.Fatal(err)
	}
	if genre != "sci-fi" || year != 1999 || status != "completed" {
		t.Fatalf("metadata lost: %s %d %s", genre, year, status)
	}
	for _, info := range []string{`{}`, `null`, `[]`, `{"releaseStatus":"bad"}`, `{"releaseStatus":null}`, `{"releaseStatus":"completed","releaseYear":1700}`, `{"releaseStatus":"completed","genre":1}`} {
		if _, err := pool.Exec(ctx, `UPDATE media_objects SET info=$1::json WHERE id=$2`, info, mediaID); err == nil {
			t.Fatalf("accepted invalid info: %s", info)
		}
	}
	voter := uuid.New()
	exec(`INSERT INTO users (id,telegram_id,first_name) VALUES ($1,100,'Voter')`, voter)
	var expected []uuid.UUID
	var newest uuid.UUID
	for i := 0; i < 9; i++ {
		author, review := uuid.New(), uuid.New()
		if i == 0 {
			newest = review
		}
		exec(`INSERT INTO users (id,telegram_id,first_name) VALUES ($1,$2,'Author')`, author, i+1)
		target, publication := mediaID, "published"
		if i == 7 {
			publication = "draft"
		}
		if i == 8 {
			target = otherID
		}
		exec(`INSERT INTO reviews (id,author_id,media_object_id,title,rating,status,likes_count,created_at) VALUES ($1,$2,$3,'Review',8,$4,$5,'2026-01-01'::timestamptz - $5::int * interval '1 second')`, review, author, target, publication, i)
		for j := 0; j < i; j++ {
			user := uuid.New()
			exec(`INSERT INTO users (id,telegram_id,first_name) VALUES ($1,$2,'Liker')`, user, 1000+i*10+j)
			exec(`INSERT INTO review_votes (review_id,user_id,is_like) VALUES ($1,$2,true)`, review, user)
		}
		if i >= 2 && i <= 6 {
			expected = append([]uuid.UUID{review}, expected...)
		}
	}
	exec(`INSERT INTO review_votes (review_id,user_id,is_like) VALUES ($1,$2,false)`, expected[0], voter)
	repo := NewReviewRepository(pool)
	items, err := repo.TopByMedia(ctx, mediaID, voter)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d reviews", len(items))
	}
	for i, item := range items {
		if item.ID != expected[i] || item.LikesCount != int64(6-i) || item.Status != "published" {
			t.Fatalf("item %d = %+v", i, item)
		}
	}
	if items[0].ViewerIsLike == nil || *items[0].ViewerIsLike {
		t.Fatal("viewer dislike missing")
	}
	if _, err := repo.TopByMedia(ctx, uuid.New(), uuid.Nil); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("missing media: %v", err)
	}
	empty := uuid.New()
	exec(`INSERT INTO media_objects (id,title,type,info) VALUES ($1,'Empty','movie','{"releaseStatus":"announced"}')`, empty)
	items, err = repo.TopByMedia(ctx, empty, uuid.Nil)
	if err != nil || len(items) != 0 {
		t.Fatalf("empty: %+v %v", items, err)
	}
	// Equal likes use creation time, then UUID as a deterministic tie breaker.
	exec(`UPDATE reviews SET likes_count=0 WHERE media_object_id=$1`, mediaID)
	items, err = repo.TopByMedia(ctx, mediaID, uuid.Nil)
	if err != nil || len(items) != 5 || items[0].ID != newest {
		t.Fatalf("creation time tie breaker: %+v %v", items, err)
	}
	exec(`UPDATE reviews SET likes_count=0,created_at='2026-01-01' WHERE media_object_id=$1`, mediaID)
	items, err = repo.TopByMedia(ctx, mediaID, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].ID.String() <= items[i].ID.String() {
			t.Fatal("unstable tie order")
		}
	}
	if err := goose.DownToContext(ctx, db, ".", 19); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT genre,release_year,release_status FROM media_objects WHERE id=$1`, mediaID).Scan(&genre, &year, &status); err != nil {
		t.Fatal(err)
	}
	if genre != "sci-fi" || year != 1999 || status != "completed" {
		t.Fatal("rollback lost metadata")
	}
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
