package write_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/command"
	"review-api/internal/modules/reviews/internal/application/query"
	"review-api/internal/modules/reviews/internal/domain"
	outboxpub "review-api/internal/modules/reviews/internal/infrastructure/outbox"
	"review-api/internal/modules/reviews/internal/infrastructure/read"
	"review-api/internal/modules/reviews/internal/infrastructure/write"
	"review-api/internal/platform/outbox"
	"review-api/internal/platform/transaction"
	"review-api/migrations"
)

func TestReviewEditorIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "test_editor_" + uuid.New().String()[:8]
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
	if err := goose.UpToContext(ctx, db, ".", 31); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}

	author, media, media2 := uuid.New(), uuid.New(), uuid.New()
	published, draft := uuid.New(), uuid.New()
	exec(`INSERT INTO users (id,telegram_id,first_name) VALUES ($1,1,'Author')`, author)
	exec(`INSERT INTO media_objects (id,title,type,info) VALUES ($1,'Matrix','movie','{"releaseStatus":"completed"}'),($2,'Other','movie','{"releaseStatus":"completed"}')`, media, media2)
	exec(`INSERT INTO reviews (id,author_id,media_object_id,title,rating,status) VALUES ($1,$2,$3,'Old',7,'published'),($4,$2,$5,'Draft',4,'draft')`, published, author, media, draft, media2)
	exec(`INSERT INTO review_votes (review_id,user_id,is_like) VALUES ($1,$2,TRUE)`, published, author)
	exec(`INSERT INTO review_blocks (id,review_id,position,title,body) VALUES
		($1,$3,2,'Минусы & <итог>','<p>||спойлер||</p>'),
		($2,$3,1,'Плюсы','<p><strong>сюжет</strong></p>')`, uuid.New(), uuid.New(), published)

	if err := goose.UpToContext(ctx, db, ".", 32); err != nil {
		t.Fatal(err)
	}

	var body, draftBody string
	var rev int64
	var pubRev *int64
	if err := pool.QueryRow(ctx, `SELECT body, draft_body, revision, published_revision FROM reviews WHERE id=$1`, published).Scan(&body, &draftBody, &rev, &pubRev); err != nil {
		t.Fatal(err)
	}
	wantBody := "<h3>Плюсы</h3><p><strong>сюжет</strong></p><h3>Минусы &amp; &lt;итог&gt;</h3><p>||спойлер||</p>"
	if body != wantBody || draftBody != wantBody || pubRev == nil || *pubRev != rev {
		t.Fatalf("migrated: body=%q draft=%q rev=%d pub=%v", body, draftBody, rev, pubRev)
	}
	if err := pool.QueryRow(ctx, `SELECT published_revision FROM reviews WHERE id=$1`, draft).Scan(&pubRev); err != nil || pubRev != nil {
		t.Fatalf("draft got a public version: %v %v", pubRev, err)
	}

	writer, reader := write.NewReviewRepository(pool), read.NewReviewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	create := command.NewCreateReviewHandler(writer, reader)
	save := command.NewSaveDraftHandler(writer, reader, tx)
	publish := command.NewPublishReviewHandler(writer, reader, tx, outboxpub.New(outbox.NewStore(pool)))
	get := query.NewGetReviewHandler(reader)

	// Migrated review keeps id, votes and spoilers.
	item, err := get.Handle(ctx, published, uuid.Nil)
	if err != nil || item.Body != wantBody || !item.HasSpoilers || item.LikesCount != 1 {
		t.Fatalf("migrated read: %+v %v", item, err)
	}
	// Existing draft is returned, not duplicated or overwritten.
	existing, created, err := create.Handle(ctx, author, media2, domain.Draft{Body: "<p>другое</p>"})
	if err != nil || created || existing.ID != draft || existing.Title != "Draft" {
		t.Fatalf("create over existing: %+v created=%v %v", existing, created, err)
	}

	// Concurrent first edit from two tabs yields one review.
	media3 := uuid.New()
	exec(`INSERT INTO media_objects (id,title,type,info) VALUES ($1,'New','movie','{"releaseStatus":"completed"}')`, media3)
	var wg sync.WaitGroup
	ids := make([]uuid.UUID, 8)
	errs := make([]error, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out, _, err := create.Handle(ctx, author, media3, domain.Draft{})
			ids[i], errs[i] = out.ID, err
		}(i)
	}
	wg.Wait()
	for i := range ids {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Fatalf("concurrent create: %v %v", ids, errs)
		}
	}
	newID := ids[0]
	if _, err := get.Handle(ctx, newID, author); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("draft visible publicly, even to author: %v", err)
	}

	// Concurrent autosave with the same revision: exactly one wins.
	r8, b := int16(8), "<p>текст</p>"
	var okCount, conflicts int
	var mu sync.Mutex
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := save.Handle(ctx, newID, author, 1, domain.Draft{Rating: &r8, Body: b})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.Is(err, contract.ErrRevisionConflict):
				conflicts++
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 || conflicts != 3 {
		t.Fatalf("revision race: ok=%d conflicts=%d", okCount, conflicts)
	}

	if _, err := publish.Handle(ctx, newID, author, 1); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("stale revision published: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := publish.Handle(ctx, newID, author, 2); err != nil {
			t.Fatal(err)
		}
	}
	countEvents := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE event_name=$1 AND aggregate_id=$2`, contract.EventPublished, newID.String()).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := countEvents(); n != 1 {
		t.Fatalf("published events=%d", n)
	}

	// Edits of a public review stay private until "Обновить публикацию".
	r3 := int16(3)
	editor, err := save.Handle(ctx, newID, author, 2, domain.Draft{Title: "Правка", Rating: &r3, Body: "<p>новое ||x||</p>"})
	if err != nil || !editor.HasUnpublishedChanges || editor.Revision != 3 {
		t.Fatalf("save edits: %+v %v", editor, err)
	}
	item, err = get.Handle(ctx, newID, uuid.Nil)
	if err != nil || item.Body != b || *item.Rating != 8 || item.HasSpoilers || item.Title != "" {
		t.Fatalf("autosave leaked to public: %+v %v", item, err)
	}
	if _, err := publish.Handle(ctx, newID, author, 3); err != nil {
		t.Fatal(err)
	}
	item, err = get.Handle(ctx, newID, uuid.Nil)
	if err != nil || item.Title != "Правка" || *item.Rating != 3 || !item.HasSpoilers {
		t.Fatalf("update publication: %+v %v", item, err)
	}
	if n := countEvents(); n != 1 {
		t.Fatalf("update re-emitted events: %d", n)
	}

	// Another user cannot read or change the working version.
	stranger := uuid.New()
	if _, err := reader.Editor(ctx, newID, stranger); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign editor read: %v", err)
	}
	if _, err := save.Handle(ctx, newID, stranger, 3, domain.Draft{}); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign save: %v", err)
	}
}
