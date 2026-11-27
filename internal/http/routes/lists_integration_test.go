package routes

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	authcontract "review-api/internal/modules/auth/contract"
	catalogmodule "review-api/internal/modules/catalog"
	listsmodule "review-api/internal/modules/lists"
	mediamodule "review-api/internal/modules/media"
	reviewsmodule "review-api/internal/modules/reviews"
	usersmodule "review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/database"
	"review-api/internal/platform/httpserver"
)

type listsTier struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type listsItem struct {
	ID string `json:"id"`
}

type listsObject struct {
	ID         string      `json:"id"`
	Format     string      `json:"format"`
	Revision   int         `json:"revision"`
	Status     string      `json:"status"`
	Moderation string      `json:"moderation"`
	Tiers      []listsTier `json:"tiers"`
	Items      []listsItem `json:"items"`
	Template   struct {
		ID string `json:"id"`
	} `json:"template"`
	CanCreateOwn bool `json:"canCreateOwn"`
}

type listsPage struct {
	Items []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"items"`
}

func TestListsHTTPIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	schema := "test_lists_http_" + uuid.New().String()[:8]
	if _, err := root.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := root.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
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

	tokens := map[string]int64{"Bearer alice": 1, "Bearer bob": 2, "Bearer carol": 3}
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, ok := tokens[c.Request().Header.Get("Authorization")]
			if !ok {
				return echo.ErrUnauthorized
			}
			identity := authcontract.TelegramIdentity{TelegramID: id}
			c.SetRequest(c.Request().WithContext(authcontract.WithIdentity(c.Request().Context(), identity)))
			return next(c)
		}
	}
	adminToken := strings.Repeat("k", 32)
	users := usersmodule.New(usersmodule.Dependencies{Pool: pool, Auth: auth})
	for id := int64(1); id <= 3; id++ {
		if err := users.EnsureUser(ctx, userscontract.TelegramProfile{TelegramID: id, FirstName: "User"}); err != nil {
			t.Fatal(err)
		}
	}
	alice, err := users.UserID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	work := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO media_objects (id, title, type, info, poster_url) VALUES ($1, 'Berserk', 'manga', '{"releaseStatus":"completed"}', 'https://cdn.example/b.jpg')`, work); err != nil {
		t.Fatal(err)
	}
	reviews := reviewsmodule.New(reviewsmodule.Dependencies{Pool: pool, Auth: auth, Users: users})
	catalog := catalogmodule.New(catalogmodule.Dependencies{Pool: pool})
	media := mediamodule.New(mediamodule.Dependencies{Pool: pool, Auth: auth, Users: users, StorageDir: t.TempDir(), PublicPrefix: "/storage", UploadsPerHour: 10})
	lists := listsmodule.New(listsmodule.Dependencies{
		Pool: pool, Auth: auth, Admin: httpserver.AdminToken(adminToken), Users: users,
		Catalog: catalog, Images: media, Reviews: reviews,
	})
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	SetupAPIRoutes(e, APIModules{Users: users, Lists: lists}, APIOptions{})

	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		switch {
		case token == "admin":
			req.Header.Set(httpserver.AdminTokenHeader, adminToken)
		case token != "":
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	check := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		rec := do(method, path, token, body)
		if rec.Code != status {
			t.Fatalf("%s %s as %q: %d %s", method, path, token, rec.Code, rec.Body.String())
		}
		return rec
	}
	decode := func(rec *httptest.ResponseRecorder, v any) {
		t.Helper()
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
	}
	mustContain := func(rec *httptest.ResponseRecorder, parts ...string) {
		t.Helper()
		for _, p := range parts {
			if !strings.Contains(rec.Body.String(), p) {
				t.Fatalf("body lacks %s: %s", p, rec.Body.String())
			}
		}
	}
	mustNotContain := func(rec *httptest.ResponseRecorder, parts ...string) {
		t.Helper()
		for _, p := range parts {
			if strings.Contains(rec.Body.String(), p) {
				t.Fatalf("body has %s: %s", p, rec.Body.String())
			}
		}
	}
	noStore := func(rec *httptest.ResponseRecorder, want bool) {
		t.Helper()
		if got := rec.Header().Get("Cache-Control") == "no-store"; got != want {
			t.Fatalf("no-store = %v, want %v", got, want)
		}
	}

	// Writes need a user; admin routes need the admin token.
	check("POST", "/list-templates", "", `{"typeCode":"manga"}`, 401)
	check("GET", "/profile/list-templates", "", "", 401)
	check("GET", "/admin/list-reports", "", "", 401)
	check("GET", "/admin/list-reports", "alice", "", 401)
	rec := check("POST", "/list-templates", "alice", `{"typeCode":"nope"}`, 422)
	mustContain(rec, `"field":"typeCode"`, `"reason":"invalid"`)

	// Without format the draft is a tier list with default tiers, shown to its author only.
	var tpl listsObject
	rec = check("POST", "/list-templates", "alice", `{"typeCode":"manga","title":"Сэйнэн"}`, 201)
	noStore(rec, true)
	decode(rec, &tpl)
	if tpl.Format != "tiers" || tpl.Status != "draft" || tpl.Revision != 1 || len(tpl.Tiers) == 0 {
		t.Fatalf("created template = %+v", tpl)
	}
	tplPath := "/list-templates/" + tpl.ID
	check("GET", tplPath, "", "", 404)
	check("GET", tplPath, "bob", "", 404)
	check("GET", "/list-templates/not-a-uuid", "", "", 404)

	tiers, _ := json.Marshal(tpl.Tiers)
	save := func(token string, revision int, status int) *httptest.ResponseRecorder {
		body := `{"revision":` + itoa(revision) + `,"title":"Сэйнэн","description":"","tiers":` + string(tiers) +
			`,"items":[{"mediaObjectId":"` + work.String() + `"}]}`
		return check("PUT", tplPath, token, body, status)
	}
	save("bob", 1, 404)
	check("DELETE", tplPath, "bob", "", 404)
	rec = save("alice", 1, 200)
	decode(rec, &tpl)
	if tpl.Revision != 2 || len(tpl.Items) != 1 {
		t.Fatalf("saved template = %+v", tpl)
	}
	mustContain(save("alice", 1, 409), `"REVISION_CONFLICT"`)
	check("POST", tplPath+"/publish", "alice", `{"visibility":"public"}`, 422)
	check("POST", tplPath+"/publish", "alice", `{"revision":2,"visibility":"unlisted"}`, 200)
	big := `{"revision":2,"title":"` + strings.Repeat("x", 130*1024) + `"}`
	check("PUT", tplPath, "alice", big, 413)

	// Guests see a published template without private fields; unlisted stays out of the section.
	rec = check("GET", tplPath, "", "", 200)
	noStore(rec, false)
	mustNotContain(rec, `"revision"`, `"status"`, `"visibility"`, `"imageId"`, `telegram`)
	mustContain(rec, `"canCreateResult":true`)
	rec = check("GET", tplPath, "alice", "", 200)
	noStore(rec, true)
	mustContain(rec, `"revision":2`, `"status":"published"`, `"visibility":"unlisted"`)
	var page listsPage
	decode(check("GET", "/list-templates", "", "", 200), &page)
	if len(page.Items) != 0 {
		t.Fatalf("unlisted template in the section: %+v", page)
	}
	check("PATCH", tplPath+"/visibility", "alice", `{"visibility":"public"}`, 200)
	decode(check("GET", "/list-templates?q=сэй", "", "", 200), &page)
	if len(page.Items) != 1 || page.Items[0].ID != tpl.ID || page.Items[0].Status != "" {
		t.Fatalf("section = %+v", page)
	}
	check("GET", "/list-templates?limit=0x", "", "", 422)
	check("GET", "/list-templates?cursor=garbage", "", "", 422)

	// The author may rank their own published template too.
	var own listsObject
	decode(check("POST", tplPath+"/results", "alice", "", 201), &own)
	check("DELETE", "/list-results/"+own.ID, "alice", "", 204)

	// A result on the template: foreign item ids are rejected, the template link is public.
	var res listsObject
	rec = check("POST", tplPath+"/results", "bob", "", 201)
	decode(rec, &res)
	resPath := "/list-results/" + res.ID
	check("PUT", resPath, "bob", `{"revision":1,"title":"Мой","entries":[{"itemId":"`+uuid.NewString()+`","tierId":"`+tpl.Tiers[0].ID+`","position":1}]}`, 422)
	check("PUT", resPath, "alice", `{"revision":1,"title":"Мой","entries":[]}`, 404)
	check("PUT", resPath, "bob", `{"revision":1,"title":"Мой","entries":[{"itemId":"`+tpl.Items[0].ID+`","tierId":"`+tpl.Tiers[0].ID+`","position":1}]}`, 200)
	check("POST", resPath+"/publish", "bob", `{"revision":2,"visibility":"public"}`, 200)
	rec = check("GET", resPath, "", "", 200)
	mustNotContain(rec, `"revision"`, `"status"`)
	decode(rec, &res)
	if res.Template.ID != tpl.ID || !res.CanCreateOwn {
		t.Fatalf("result template link = %+v", res)
	}
	decode(check("GET", "/profile/list-results", "bob", "", 200), &page)
	if len(page.Items) != 1 || page.Items[0].Status != "published" {
		t.Fatalf("own results = %+v", page)
	}

	// A closed profile hides the profile tab, not the section.
	if _, err := pool.Exec(ctx, `INSERT INTO user_settings (user_id, language, is_profile_private) VALUES ($1, 'ru', TRUE)
		ON CONFLICT (user_id) DO UPDATE SET is_profile_private = TRUE`, alice); err != nil {
		t.Fatal(err)
	}
	check("GET", "/profiles/"+alice.String()+"/list-templates", "", "", 404)
	check("GET", "/profiles/"+alice.String()+"/list-templates", "alice", "", 200)
	decode(check("GET", "/list-templates", "", "", 200), &page)
	if len(page.Items) != 1 {
		t.Fatalf("closed profile hid the section: %+v", page)
	}

	// Reports and moderation.
	check("POST", tplPath+"/reports", "", `{"reasonCode":"spam"}`, 401)
	check("POST", tplPath+"/reports", "alice", `{"reasonCode":"spam"}`, 403)
	check("POST", tplPath+"/reports", "bob", `{"reasonCode":"nope"}`, 422)
	check("POST", tplPath+"/reports", "bob", `{"reasonCode":"spam"}`, 201)
	mustContain(check("POST", tplPath+"/reports", "bob", `{"reasonCode":"spam"}`, 409), `"ALREADY_REPORTED"`)
	var queue struct {
		Items []struct {
			ID       string `json:"id"`
			TargetID string `json:"targetId"`
		} `json:"items"`
	}
	rec = check("GET", "/admin/list-reports", "admin", "", 200)
	noStore(rec, true)
	decode(rec, &queue)
	if len(queue.Items) != 1 || queue.Items[0].TargetID != tpl.ID {
		t.Fatalf("queue = %+v", queue)
	}
	check("POST", "/admin/list-templates/"+tpl.ID+"/hide", "admin", "", 204)
	check("GET", resPath, "", "", 404)
	check("GET", tplPath, "carol", "", 404)
	mustContain(check("GET", tplPath, "alice", "", 200), `"moderation":"hidden"`)
	mustContain(check("POST", tplPath+"/results", "alice", "", 403), `"HIDDEN_BY_MODERATOR"`)
	check("POST", "/admin/list-templates/"+tpl.ID+"/unhide", "admin", "", 204)
	check("GET", resPath, "", "", 200)
	check("POST", "/admin/list-reports/"+queue.Items[0].ID+"/resolve", "admin", "", 200)
	check("POST", "/admin/list-reports/"+queue.Items[0].ID+"/resolve", "admin", "", 409)
	check("POST", "/admin/list-results/"+uuid.NewString()+"/hide", "admin", "", 404)

	// Deleting the template keeps the result; the link and "make your own" go away.
	check("DELETE", tplPath, "alice", "", 204)
	check("GET", tplPath, "alice", "", 404)
	var orphan listsObject
	decode(check("GET", resPath, "carol", "", 200), &orphan)
	if orphan.Template.ID != "" || orphan.CanCreateOwn {
		t.Fatalf("result of a deleted template = %+v", orphan)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
