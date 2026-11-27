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
	feedmodule "review-api/internal/modules/feed"
	listsmodule "review-api/internal/modules/lists"
	mediamodule "review-api/internal/modules/media"
	reviewsmodule "review-api/internal/modules/reviews"
	usersmodule "review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/database"
	"review-api/internal/platform/httpserver"
)

type feedPage struct {
	Items []struct {
		Kind          string `json:"kind"`
		ID            string `json:"id"`
		Title         string `json:"title"`
		Rating        *int   `json:"rating"`
		LikesCount    int64  `json:"likesCount"`
		DislikesCount int64  `json:"dislikesCount"`
		ViewsCount    int64  `json:"viewsCount"`
		ViewerIsLike  *bool  `json:"viewerIsLike"`
		MediaObject   *struct {
			ID string `json:"id"`
		} `json:"mediaObject"`
		List *struct {
			TypeCode      string `json:"typeCode"`
			Format        string `json:"format"`
			CoverURL      string `json:"coverUrl"`
			TemplateTitle string `json:"templateTitle"`
		} `json:"list"`
	} `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func TestFeedWithListsHTTPIntegration(t *testing.T) {
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
	schema := "test_feed_lists_" + uuid.New().String()[:8]
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
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
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
	users := usersmodule.New(usersmodule.Dependencies{Pool: pool, Auth: auth})
	ids := make([]uuid.UUID, 3)
	for i := range ids {
		tgID := int64(i + 1)
		if err := users.EnsureUser(ctx, userscontract.TelegramProfile{TelegramID: tgID, FirstName: "User"}); err != nil {
			t.Fatal(err)
		}
		if ids[i], err = users.UserID(ctx, tgID); err != nil {
			t.Fatal(err)
		}
	}
	alice, bob, carol := ids[0], ids[1], ids[2]
	work := uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES ($1, 'Berserk', 'manga', '{"releaseStatus":"completed"}', 'https://cdn.example/b.jpg')`, work)
	// The review is older than the list result, so "new" puts the list first.
	review := uuid.New()
	exec(`INSERT INTO reviews (id, author_id, media_object_id, title, rating, status, created_at)
		VALUES ($1, $2, $3, 'Обзор', 8, 'published', NOW() - INTERVAL '1 day')`, review, alice, work)
	exec(`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`, carol, bob)

	reviews := reviewsmodule.New(reviewsmodule.Dependencies{Pool: pool, Auth: auth, Users: users})
	catalog := catalogmodule.New(catalogmodule.Dependencies{Pool: pool})
	media := mediamodule.New(mediamodule.Dependencies{Pool: pool, Auth: auth, Users: users, StorageDir: t.TempDir(), PublicPrefix: "/storage", UploadsPerHour: 10})
	lists := listsmodule.New(listsmodule.Dependencies{Pool: pool, Auth: auth, Users: users, Catalog: catalog, Images: media, Reviews: reviews})
	feed := feedmodule.New(feedmodule.Dependencies{Pool: pool, Auth: auth, Users: users})
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	SetupAPIRoutes(e, APIModules{Users: users, Lists: lists, Feed: feed}, APIOptions{})

	check := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
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
	getFeed := func(query, token string) feedPage {
		t.Helper()
		var p feedPage
		decode(check("GET", "/feed"+query, token, "", 200), &p)
		return p
	}

	// alice publishes a ranked template, bob publishes a result on it.
	var tpl listsObject
	decode(check("POST", "/list-templates", "alice", `{"typeCode":"manga","format":"ranked","title":"Сэйнэн"}`, 201), &tpl)
	decode(check("PUT", "/list-templates/"+tpl.ID, "alice",
		`{"revision":1,"title":"Сэйнэн","tiers":[],"items":[{"mediaObjectId":"`+work.String()+`"}]}`, 200), &tpl)
	check("POST", "/list-templates/"+tpl.ID+"/publish", "alice", `{"revision":2}`, 200)
	var res listsObject
	decode(check("POST", "/list-templates/"+tpl.ID+"/results", "bob", "", 201), &res)
	resPath := "/list-results/" + res.ID
	decode(check("PUT", resPath, "bob", `{"revision":1,"title":"Мой топ","entries":[{"itemId":"`+res.Items[0].ID+`","position":1}]}`, 200), &res)

	// A draft takes no reactions and is not in the feed.
	check("PUT", resPath+"/vote", "carol", `{"isLike":true}`, 404)
	if p := getFeed("?kind=lists", ""); len(p.Items) != 0 {
		t.Fatalf("draft in the feed: %+v", p)
	}
	check("POST", resPath+"/publish", "bob", `{"revision":2}`, 200)

	// Reactions on the published result.
	check("PUT", resPath+"/vote", "", `{"isLike":true}`, 401)
	check("PUT", resPath+"/vote", "carol", `{}`, 422)
	check("PUT", resPath+"/vote", "carol", `{"isLike":true}`, 204)
	check("PUT", resPath+"/view", "carol", "", 204)
	check("PUT", resPath+"/view", "bob", "", 204) // the author's own view is not counted
	check("PUT", "/list-results/"+uuid.NewString()+"/view", "carol", "", 404)
	var page struct {
		LikesCount   int64 `json:"likesCount"`
		ViewsCount   int64 `json:"viewsCount"`
		ViewerIsLike *bool `json:"viewerIsLike"`
	}
	decode(check("GET", resPath, "carol", "", 200), &page)
	if page.LikesCount != 1 || page.ViewsCount != 1 || page.ViewerIsLike == nil || !*page.ViewerIsLike {
		t.Fatalf("result page = %+v", page)
	}

	// Comments.
	check("POST", resPath+"/comments", "", `{"body":"hi"}`, 401)
	check("POST", resPath+"/comments", "carol", `{"body":"   "}`, 422)
	var comment struct {
		ID       string  `json:"id"`
		ResultID string  `json:"resultId"`
		ParentID *string `json:"parentId"`
		Body     string  `json:"body"`
	}
	decode(check("POST", resPath+"/comments", "carol", `{"body":"Хороший топ"}`, 201), &comment)
	if comment.ResultID != res.ID || comment.ParentID != nil || comment.Body != "Хороший топ" {
		t.Fatalf("comment = %+v", comment)
	}
	check("POST", resPath+"/comments", "bob", `{"body":"Спасибо","parentId":"`+comment.ID+`"}`, 201)
	check("POST", resPath+"/comments", "bob", `{"body":"x","parentId":"`+uuid.NewString()+`"}`, 404)
	rec := check("GET", resPath+"/comments", "", "", 200)
	if !strings.Contains(rec.Body.String(), `"hasReplies":true`) || strings.Contains(rec.Body.String(), "Спасибо") {
		t.Fatalf("top-level comments: %s", rec.Body.String())
	}
	rec = check("GET", resPath+"/comments?parentId="+comment.ID, "", "", 200)
	if !strings.Contains(rec.Body.String(), "Спасибо") {
		t.Fatalf("replies: %s", rec.Body.String())
	}
	check("GET", resPath+"/comments?limit=0x", "", "", 422)

	// The mixed feed: newest first, both kinds, each with its own fields.
	p := getFeed("", "carol")
	if len(p.Items) != 2 || p.Items[0].Kind != "list" || p.Items[1].Kind != "review" {
		t.Fatalf("mixed feed = %+v", p)
	}
	list, rev := p.Items[0], p.Items[1]
	if list.ID != res.ID || list.Rating != nil || list.MediaObject != nil || list.List == nil ||
		list.List.Format != "ranked" || list.List.TypeCode != "manga" || list.List.TemplateTitle != "Сэйнэн" ||
		list.List.CoverURL != "https://cdn.example/b.jpg" || list.LikesCount != 1 || list.ViewsCount != 1 ||
		list.ViewerIsLike == nil || !*list.ViewerIsLike {
		t.Fatalf("list item = %+v", list)
	}
	if rev.ID != review.String() || rev.Rating == nil || *rev.Rating != 8 || rev.MediaObject == nil || rev.List != nil {
		t.Fatalf("review item = %+v", rev)
	}

	// Filters.
	if p := getFeed("?kind=reviews", ""); len(p.Items) != 1 || p.Items[0].Kind != "review" {
		t.Fatalf("kind=reviews = %+v", p)
	}
	if p := getFeed("?kind=lists", ""); len(p.Items) != 1 || p.Items[0].Kind != "list" {
		t.Fatalf("kind=lists = %+v", p)
	}
	check("GET", "/feed?kind=templates", "", "", 422)
	if p := getFeed("?type=game", ""); len(p.Items) != 0 {
		t.Fatalf("type=game = %+v", p)
	}
	if p := getFeed("?type=manga", ""); len(p.Items) != 2 {
		t.Fatalf("type=manga = %+v", p)
	}
	if p := getFeed("?mode=following", "carol"); len(p.Items) != 1 || p.Items[0].Kind != "list" {
		t.Fatalf("following = %+v", p)
	}

	// Sorts page through both kinds with one cursor: the review gets more views than the list.
	exec(`UPDATE reviews SET views_count = 5 WHERE id = $1`, review)
	first := getFeed("?sort=views&limit=1", "")
	if len(first.Items) != 1 || first.Items[0].Kind != "review" || first.NextCursor == nil {
		t.Fatalf("views page 1 = %+v", first)
	}
	second := getFeed("?sort=views&limit=1&cursor="+*first.NextCursor, "")
	if len(second.Items) != 1 || second.Items[0].Kind != "list" || second.NextCursor != nil {
		t.Fatalf("views page 2 = %+v", second)
	}
	if p := getFeed("?sort=likes", ""); len(p.Items) != 2 || p.Items[0].Kind != "list" {
		t.Fatalf("likes = %+v", p)
	}

	// A result of a mixed template (NULL type): found by type=mixed only, never with reviews.
	exec(`UPDATE list_template_versions SET type_code = NULL WHERE id = (SELECT version_id FROM list_results WHERE id = $1)`, res.ID)
	if p := getFeed("?type=mixed", ""); len(p.Items) != 1 || p.Items[0].List == nil || p.Items[0].List.TypeCode != "mixed" {
		t.Fatalf("type=mixed = %+v", p)
	}
	if p := getFeed("?type=manga", ""); len(p.Items) != 1 || p.Items[0].Kind != "review" {
		t.Fatalf("type=manga with a mixed list = %+v", p)
	}
	if p := getFeed("?kind=reviews&type=mixed", ""); len(p.Items) != 0 {
		t.Fatalf("kind=reviews&type=mixed = %+v", p)
	}

	// Unlisted and hidden results leave the feed but keep their counters.
	check("PATCH", resPath+"/visibility", "bob", `{"visibility":"unlisted"}`, 200)
	if p := getFeed("?kind=lists", ""); len(p.Items) != 0 {
		t.Fatalf("unlisted in the feed: %+v", p)
	}
	check("PUT", resPath+"/vote", "alice", `{"isLike":false}`, 204)
}
