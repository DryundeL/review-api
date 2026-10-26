package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/sitemap"
	"review-api/internal/platform/database"
	"review-api/internal/platform/httpserver"
)

func TestSitemapIntegration(t *testing.T) {
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
	schema := "test_sitemap_" + uuid.New().String()[:8]
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
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Deterministic IDs also exercise pagination across hidden rows.
	id := func(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
	for i := 1; i <= 5; i++ {
		exec(`INSERT INTO users (id, telegram_id, first_name) VALUES ($1, $2, 'Author')`, id(i), i)
	}
	exec(`INSERT INTO user_settings (user_id, language, is_profile_private) VALUES ($1,'en',false),($2,'en',true),($3,'en',false)`, id(1), id(2), id(4))
	for i := 11; i <= 15; i++ {
		exec(`INSERT INTO media_objects (id,title,type,info) VALUES ($1,'Work','movie','{"releaseStatus":"completed"}')`, id(i))
	}
	exec(`INSERT INTO reviews (id,author_id,media_object_id,title,rating,status) VALUES
		($1,$4,$6,'Published',8,'published'),
		($2,$4,$7,'Draft',8,'draft'),
		($3,$5,$6,'Published by private author',8,'published')`, id(21), id(22), id(23), id(1), id(2), id(11), id(12))
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	SetupAPIRoutes(e, APIModules{Sitemap: sitemap.New(sitemap.Dependencies{Pool: pool})}, APIOptions{})
	type page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		NextCursor string `json:"nextCursor"`
	}
	get := func(path string, status int) page {
		t.Helper()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != status {
			t.Fatalf("%s: status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		var p page
		if status == 200 {
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing no-store")
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Items == nil {
				t.Fatal("items must be an array")
			}
		}
		return p
	}
	for entity, expected := range map[string][]string{
		"media-objects": {id(11), id(12), id(13), id(14), id(15)},
		"reviews":       {id(21), id(23)},
		"authors":       {id(1), id(3), id(4), id(5)},
	} {
		t.Run(entity, func(t *testing.T) {
			got := []string{}
			cursor := ""
			for n := 0; ; n++ {
				if n > 5 {
					t.Fatal("pagination did not terminate")
				}
				p := get("/v1/sitemap/"+entity+"?limit=2&cursor="+cursor, 200)
				for _, item := range p.Items {
					got = append(got, item.ID)
				}
				if p.NextCursor == "" {
					break
				}
				cursor = p.NextCursor
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("IDs=%v want=%v", got, expected)
			}
		})
	}
	for _, path := range []string{
		"unknown", "reviews?limit=0", "reviews?limit=1001", "reviews?limit=abc", "reviews?cursor=invalid",
	} {
		get("/v1/sitemap/"+path, 422)
	}
	p := get("/v1/sitemap/media-objects?cursor="+id(99), 200)
	if len(p.Items) != 0 || p.NextCursor != "" {
		t.Fatalf("last page: %+v", p)
	}
	// Deleted boundary rows do not invalidate a cursor.
	exec(`DELETE FROM media_objects WHERE id=$1`, id(13))
	p = get("/v1/sitemap/media-objects?cursor="+id(13), 200)
	if len(p.Items) != 2 || p.Items[0].ID != id(14) {
		t.Fatalf("deleted cursor: %+v", p)
	}
	// Visibility is evaluated on every request, including subsequent pages.
	exec(`UPDATE user_settings SET is_profile_private=true WHERE user_id=$1`, id(4))
	p = get("/v1/sitemap/authors?cursor="+id(3), 200)
	if len(p.Items) != 1 || p.Items[0].ID != id(5) {
		t.Fatalf("private profile leaked: %+v", p)
	}
}
