package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	authcontract "review-api/internal/modules/auth/contract"
	notificationsmodule "review-api/internal/modules/notifications"
	reviewsmodule "review-api/internal/modules/reviews"
	reviewscontract "review-api/internal/modules/reviews/contract"
	usersmodule "review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/database"
	"review-api/internal/platform/eventbus"
	"review-api/internal/platform/httpserver"
)

type commentTestBot struct {
	calls int
	fail  bool
}

func (b *commentTestBot) SendMessage(context.Context, int64, string, any) error {
	b.calls++
	if b.fail {
		return errors.New("test bot failure")
	}
	return nil
}

func TestCommentsIntegration(t *testing.T) {
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
	schema := "test_comments_" + uuid.New().String()[:8]
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
	cfg.ConnConfig.RuntimeParams["application_name"] = schema
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
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, err := strconv.ParseInt(strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer "), 10, 64)
			if err != nil || id < 1 || id > 3 {
				return echo.ErrUnauthorized
			}
			c.SetRequest(c.Request().WithContext(authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: id})))
			return next(c)
		}
	}
	users := usersmodule.New(usersmodule.Dependencies{Pool: pool, Auth: auth})
	var ids [3]uuid.UUID
	for i := range ids {
		tg := int64(i + 1)
		if err := users.EnsureUser(ctx, userscontract.TelegramProfile{TelegramID: tg, FirstName: "User"}); err != nil {
			t.Fatal(err)
		}
		ids[i], err = users.UserID(ctx, tg)
		if err != nil {
			t.Fatal(err)
		}
	}
	reviews := reviewsmodule.New(reviewsmodule.Dependencies{Pool: pool, Auth: auth, Users: users})
	bot := &commentTestBot{}
	notifications := notificationsmodule.New(notificationsmodule.Dependencies{Pool: pool, Auth: auth, Users: users, Reviews: reviews, Bot: bot})
	dispatcher := eventbus.NewDispatcher()
	notifications.RegisterConsumers(dispatcher)
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	SetupAPIRoutes(e, APIModules{Users: users, Reviews: reviews, Notifications: notifications}, APIOptions{})
	request := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, status, rec.Body.String())
		}
		return rec
	}
	media, review, other, draft := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO media_objects(id,title,type,info) VALUES($1,'Movie','movie','{"releaseStatus":"completed"}')`, media)
	exec(`INSERT INTO reviews(id,author_id,media_object_id,title,rating,status) VALUES($1,$4,$5,'Review',8,'published'),($2,$6,$5,'Other',7,'published'),($3,$4,$5,'Draft',6,'draft')`, review, other, draft, ids[0], media, ids[2])
	path := "/reviews/" + review.String() + "/comments"
	type comment struct {
		ID         uuid.UUID  `json:"id"`
		ParentID   *uuid.UUID `json:"parentId"`
		Body       string     `json:"body"`
		HasReplies bool       `json:"hasReplies"`
		Author     struct {
			ID uuid.UUID `json:"id"`
		} `json:"author"`
	}
	create := func(token, body string, parent *uuid.UUID) comment {
		t.Helper()
		data, _ := json.Marshal(map[string]any{"body": body, "parentId": parent, "authorId": ids[0]})
		rec := request("POST", path, token, string(data), 201)
		var c comment
		if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	type page struct {
		Items      []comment `json:"items"`
		NextCursor *string   `json:"nextCursor"`
	}
	list := func(suffix string) page {
		t.Helper()
		rec := request("GET", path+suffix, "", "", 200)
		var p page
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	root := create("2", "  Привет 🙂  ", nil)
	if root.Body != "Привет 🙂" || root.Author.ID != ids[1] || root.ParentID != nil || root.HasReplies {
		t.Fatalf("root=%+v", root)
	}
	reply := create("3", "Ответ", &root.ID)
	nested := create("1", "Вложенный ответ", &reply.ID)
	if nested.ParentID == nil || *nested.ParentID != reply.ID {
		t.Fatalf("nested=%+v", nested)
	}
	for _, tc := range []struct {
		suffix     string
		id         uuid.UUID
		hasReplies bool
	}{
		{"", root.ID, true}, {"?parentId=" + root.ID.String(), reply.ID, true}, {"?parentId=" + reply.ID.String(), nested.ID, false},
	} {
		p := list(tc.suffix)
		if len(p.Items) != 1 || p.Items[0].ID != tc.id || p.Items[0].HasReplies != tc.hasReplies {
			t.Fatalf("page=%+v", p)
		}
	}
	// Drain real persisted outbox payloads twice to exercise notification deduplication.
	dispatch := func(id uuid.UUID) {
		t.Helper()
		var payload []byte
		if err := pool.QueryRow(ctx, `SELECT payload FROM outbox_messages WHERE event_name=$1 AND payload->>'commentId'=$2`, reviewscontract.EventCommentCreated, id.String()).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := dispatcher.Publish(ctx, reviewscontract.EventCommentCreated, payload); err != nil {
				t.Fatal(err)
			}
		}
	}
	dispatch(root.ID)
	dispatch(reply.ID)
	dispatch(nested.ID)
	if n := count(`SELECT COUNT(*) FROM notifications`); n != 3 || bot.calls != 3 {
		t.Fatalf("inbox=%d bot=%d", n, bot.calls)
	}
	for _, tc := range []struct {
		id, recipient uuid.UUID
		kind          string
	}{
		{root.ID, ids[0], "comment"}, {reply.ID, ids[1], "comment_reply"}, {nested.ID, ids[2], "comment_reply"},
	} {
		if n := count(`SELECT COUNT(*) FROM notifications WHERE comment_id=$1 AND user_id=$2 AND type_name=$3`, tc.id, tc.recipient, tc.kind); n != 1 {
			t.Fatalf("wrong recipient: %+v", tc)
		}
	}
	rec := request("GET", "/notifications", "2", "", 200)
	var inbox struct {
		Items []struct {
			CommentID uuid.UUID  `json:"commentId"`
			ParentID  *uuid.UUID `json:"parentCommentId"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &inbox); err != nil {
		t.Fatal(err)
	}
	if len(inbox.Items) != 1 || inbox.Items[0].CommentID != reply.ID || inbox.Items[0].ParentID == nil || *inbox.Items[0].ParentID != root.ID {
		t.Fatalf("inbox=%s", rec.Body.String())
	}
	self := create("2", "Self", &root.ID)
	dispatch(self.ID)
	ownerSelf := create("1", "Own review", nil)
	dispatch(ownerSelf.ID)
	if n := count(`SELECT COUNT(*) FROM notifications`); n != 3 || bot.calls != 3 {
		t.Fatalf("self inbox=%d bot=%d", n, bot.calls)
	}
	request("PATCH", "/notification-settings", "1", `{"items":[{"name":"comment","enabled":false}]}`, 200)
	disabled := create("2", "Disabled", nil)
	dispatch(disabled.ID)
	if n := count(`SELECT COUNT(*) FROM notifications`); n != 3 {
		t.Fatalf("disabled inbox=%d", n)
	}
	request("PATCH", "/notification-settings", "2", `{"items":[{"name":"comment_reply","enabled":false}]}`, 200)
	disabledReply := create("3", "Disabled reply", &root.ID)
	dispatch(disabledReply.ID)
	if n := count(`SELECT COUNT(*) FROM notifications`); n != 3 {
		t.Fatalf("disabled reply inbox=%d", n)
	}
	request("PATCH", "/notification-settings", "2", `{"items":[{"name":"comment_reply","enabled":true}]}`, 200)
	if err := users.SetAllowNotifications(ctx, 2, false); err != nil {
		t.Fatal(err)
	}
	muted := create("3", "Muted Telegram", &root.ID)
	dispatch(muted.ID)
	if n := count(`SELECT COUNT(*) FROM notifications`); n != 4 || bot.calls != 3 {
		t.Fatalf("muted inbox=%d bot=%d", n, bot.calls)
	}
	if err := users.SetAllowNotifications(ctx, 2, true); err != nil {
		t.Fatal(err)
	}
	bot.fail = true
	failed := create("3", "Failed Telegram", &root.ID)
	dispatch(failed.ID)
	if n := count(`SELECT COUNT(*) FROM notifications`); n != 5 || bot.calls != 4 {
		t.Fatalf("failed inbox=%d bot=%d", n, bot.calls)
	}
	bot.fail = false

	t.Run("invalid requests", func(t *testing.T) {
		request("POST", path, "", `{"body":"no auth"}`, 401)
		request("GET", path, "invalid", "", 401)
		for _, body := range []string{`{`, `[]`, `{"body":1}`} {
			request("POST", path, "2", body, 400)
		}
		for _, body := range []string{`{}`, `{"body":" "}`, `{"body":"ok","parentId":"bad"}`, `{"body":"\u0000"}`, `{"body":"` + strings.Repeat("🙂", 2001) + `"}`} {
			request("POST", path, "2", body, 422)
		}
		request("POST", path, "2", `{"body":"`+string([]byte{0xff})+`"}`, 422)
		request("POST", path, "2", strings.Repeat(" ", 32769), 400)
		for _, suffix := range []string{"?limit=0", "?limit=51", "?limit=bad", "?limit=", "?parentId=", "?parentId=bad", "?cursor=bad", "?limit=1&limit=2"} {
			request("GET", path+suffix, "", "", 422)
		}
		request("GET", path+"?parentId="+uuid.NewString(), "", "", 404)
		request("POST", "/reviews/"+other.String()+"/comments", "2", fmt.Sprintf(`{"body":"cross-review","parentId":%q}`, root.ID), 404)
		request("GET", "/reviews/"+other.String()+"/comments?parentId="+root.ID.String(), "", "", 404)
		for _, id := range []uuid.UUID{draft, uuid.New()} {
			request("GET", "/reviews/"+id.String()+"/comments", "", "", 404)
			request("POST", "/reviews/"+id.String()+"/comments", "1", `{"body":"hidden"}`, 404)
		}
	})

	t.Run("pagination with tied timestamps and deep replies", func(t *testing.T) {
		// Force ties to prove the UUID tiebreaker is used for roots and replies.
		for i := 0; i < 12; i++ {
			create("2", fmt.Sprintf("Root %d", i), nil)
			create("3", fmt.Sprintf("Reply %d", i), &root.ID)
		}
		parent := nested.ID
		for i := 0; i < 20; i++ {
			c := create("2", "Deep", &parent)
			parent = c.ID
		}
		exec(`UPDATE review_comments SET created_at='2026-01-01T00:00:00Z' WHERE review_id=$1`, review)
		var rootCursor string
		for _, parentID := range []*uuid.UUID{nil, &root.ID, &reply.ID} {
			query := "?limit=2"
			if parentID != nil {
				query += "&parentId=" + parentID.String()
			}
			seen := map[uuid.UUID]bool{}
			cursor := ""
			for n := 0; ; n++ {
				if n > 100 {
					t.Fatal("pagination did not finish")
				}
				suffix := query
				if cursor != "" {
					suffix += "&cursor=" + url.QueryEscape(cursor)
				}
				p := list(suffix)
				if len(p.Items) > 2 {
					t.Fatal("unbounded page")
				}
				for _, c := range p.Items {
					if seen[c.ID] {
						t.Fatal("duplicate comment")
					}
					seen[c.ID] = true
				}
				if p.NextCursor == nil {
					break
				}
				cursor = *p.NextCursor
				if parentID == nil {
					rootCursor = cursor
				}
			}
			filter := `parent_id IS NULL`
			args := []any{review}
			if parentID != nil {
				filter = `parent_id=$2`
				args = append(args, *parentID)
			}
			if want := count(`SELECT COUNT(*) FROM review_comments WHERE review_id=$1 AND `+filter, args...); len(seen) != want {
				t.Fatalf("seen=%d want=%d", len(seen), want)
			}
		}
		request("GET", path+"?parentId="+root.ID.String()+"&cursor="+url.QueryEscape(rootCursor), "", "", 422)
		request("GET", "/reviews/"+other.String()+"/comments?cursor="+url.QueryEscape(rootCursor), "", "", 422)
		empty := list("?parentId=" + parent.String())
		if len(empty.Items) != 0 || empty.NextCursor != nil {
			t.Fatalf("empty=%+v", empty)
		}
	})
	t.Run("foreign key and outbox rollback", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO review_comments(id,review_id,author_id,parent_id,body,created_at) VALUES($1,$2,$3,$4,'bad',NOW())`, uuid.New(), other, ids[1], root.ID); err == nil {
			t.Fatal("accepted foreign parent")
		}
		before := count(`SELECT COUNT(*) FROM review_comments`)
		exec(`ALTER TABLE outbox_messages ADD CONSTRAINT test_reject_comment CHECK(event_name <> 'review.comment_created') NOT VALID`)
		request("POST", path, "2", `{"body":"must rollback"}`, 500)
		exec(`ALTER TABLE outbox_messages DROP CONSTRAINT test_reject_comment`)
		if count(`SELECT COUNT(*) FROM review_comments`) != before {
			t.Fatal("comment persisted without outbox")
		}
	})

	t.Run("concurrent unpublish blocks comment creation", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `UPDATE reviews SET status='draft' WHERE id=$1`, review); err != nil {
			t.Fatal(err)
		}
		before := count(`SELECT COUNT(*) FROM review_comments`)
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			req := httptest.NewRequest("POST", "/v1"+path, strings.NewReader(`{"body":"racing comment"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer 2")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			done <- rec
		}()
		deadline := time.Now().Add(3 * time.Second)
		for count(`SELECT COUNT(*) FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND query LIKE 'SELECT status FROM reviews%'`, schema) == 0 {
			if time.Now().After(deadline) {
				t.Fatal("comment creation did not acquire the review lock")
			}
			time.Sleep(5 * time.Millisecond)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case rec := <-done:
			if rec.Code != 404 {
				t.Fatalf("race status=%d body=%s", rec.Code, rec.Body.String())
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if count(`SELECT COUNT(*) FROM review_comments`) != before {
			t.Fatal("comment created after unpublish")
		}
		exec(`UPDATE reviews SET status='published' WHERE id=$1`, review)
	})
	pending := create("2", "Will hide", nil)
	exec(`UPDATE reviews SET status='draft' WHERE id=$1`, review)
	before := count(`SELECT COUNT(*) FROM notifications`)
	dispatch(pending.ID)
	request("GET", path, "1", "", 404)
	request("POST", path, "2", `{"body":"hidden"}`, 404)
	if count(`SELECT COUNT(*) FROM notifications`) != before {
		t.Fatal("notified hidden review")
	}
	exec(`DELETE FROM reviews WHERE id=$1`, review)
	dispatch(pending.ID)
	if count(`SELECT COUNT(*) FROM review_comments WHERE review_id=$1`, review) != 0 {
		t.Fatal("comments not cascaded")
	}
	// Both migrations roll back and reapply; existing notification types/settings survive.
	for i := 0; i < 2; i++ {
		if err := database.MigrateDown(ctx, pool); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if count(`SELECT COUNT(*) FROM notification_types WHERE name IN ('like','follow','follow_request')`) != 3 {
		t.Fatal("lost existing notification types")
	}
}
