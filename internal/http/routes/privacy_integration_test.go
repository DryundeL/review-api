package routes

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	authcontract "review-api/internal/modules/auth/contract"
	feedmodule "review-api/internal/modules/feed"
	gamificationmodule "review-api/internal/modules/gamification"
	notificationsmodule "review-api/internal/modules/notifications"
	reviewsmodule "review-api/internal/modules/reviews"
	socialmodule "review-api/internal/modules/social"
	usersmodule "review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/database"
	"review-api/internal/platform/eventbus"
	"review-api/internal/platform/httpserver"
)

type privacyTestBot struct{ calls int }

func (b *privacyTestBot) SendMessage(context.Context, int64, string, any) error {
	b.calls++
	return nil
}

func TestPrivacyAndFollowRequestsIntegration(t *testing.T) {
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
	schema := "test_privacy_" + uuid.New().String()[:8]
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
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var id int64
			switch c.Request().Header.Get("Authorization") {
			case "Bearer owner":
				id = 1
			case "Bearer follower":
				id = 2
			case "Bearer stranger":
				id = 3
			default:
				return echo.ErrUnauthorized
			}
			identity := authcontract.TelegramIdentity{TelegramID: id}
			c.SetRequest(c.Request().WithContext(authcontract.WithIdentity(c.Request().Context(), identity)))
			return next(c)
		}
	}
	users := usersmodule.New(usersmodule.Dependencies{Pool: pool, Auth: auth})
	for id := int64(1); id <= 3; id++ {
		if err := users.EnsureUser(ctx, userscontract.TelegramProfile{TelegramID: id, FirstName: "User"}); err != nil {
			t.Fatal(err)
		}
	}
	owner, err := users.UserID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	follower, err := users.UserID(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	reviews := reviewsmodule.New(reviewsmodule.Dependencies{Pool: pool, Auth: auth, Users: users})
	bot := &privacyTestBot{}
	notifications := notificationsmodule.New(notificationsmodule.Dependencies{Pool: pool, Auth: auth, Users: users, Reviews: reviews, Bot: bot})
	dispatcher := eventbus.NewDispatcher()
	notifications.RegisterConsumers(dispatcher)
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	SetupAPIRoutes(e, APIModules{Users: users, Reviews: reviews, Notifications: notifications,
		Social:       socialmodule.New(socialmodule.Dependencies{Pool: pool, Auth: auth, Users: users}),
		Gamification: gamificationmodule.New(gamificationmodule.Dependencies{Pool: pool, Auth: auth, Users: users, Reviews: reviews}),
		Feed:         feedmodule.New(feedmodule.Dependencies{Pool: pool, Auth: auth, Users: users}),
	}, APIOptions{})
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	check := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		rec := request(method, path, token, body)
		if rec.Code != status {
			t.Fatalf("%s %s as %s: %d %s", method, path, token, rec.Code, rec.Body.String())
		}
		return rec
	}
	path := "/profiles/" + owner.String()
	state := func(want string) {
		t.Helper()
		rec := check("GET", path+"/follow-status", "follower", "", 200)
		if !strings.Contains(rec.Body.String(), `"followStatus":"`+want+`"`) {
			t.Fatalf("state=%s", rec.Body.String())
		}
	}
	check("GET", path, "", "", 200)
	check("PATCH", "/settings", "owner", `{"isProfilePrivate":true}`, 200)
	rec := check("GET", "/settings", "owner", "", 200)
	if !strings.Contains(rec.Body.String(), `"isProfilePrivate":true`) {
		t.Fatal(rec.Body.String())
	}
	// Telegram refreshes must preserve the privacy preference.
	if err := users.EnsureUser(ctx, userscontract.TelegramProfile{TelegramID: 1, FirstName: "Updated"}); err != nil {
		t.Fatal(err)
	}
	privatePaths := []string{"", "/reviews", "/following", "/followers", "/achievements"}
	for _, suffix := range privatePaths {
		check("GET", path+suffix, "", "", 404)
		check("GET", path+suffix, "owner", "", 200)
	}
	if _, err := users.UserRef(ctx, owner); err != nil {
		t.Fatalf("private user's internal reference broken: %v", err)
	}
	state("none")
	// A request must not survive a failure to record its notification event.
	if _, err := pool.Exec(ctx, `ALTER TABLE outbox_messages ADD CONSTRAINT test_request_event_failure CHECK (event_name <> 'user.follow_requested')`); err != nil {
		t.Fatal(err)
	}
	check("POST", path+"/follow-requests", "follower", "", 500)
	state("none")
	if _, err := pool.Exec(ctx, `ALTER TABLE outbox_messages DROP CONSTRAINT test_request_event_failure`); err != nil {
		t.Fatal(err)
	}
	dispatchLatestRequest := func() {
		t.Helper()
		var payload []byte
		if err := pool.QueryRow(ctx, `SELECT payload FROM outbox_messages WHERE event_name='user.follow_requested' ORDER BY created_at DESC,id DESC LIMIT 1`).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		// A retry of the same outbox event must not duplicate either inbox or bot message.
		for i := 0; i < 2; i++ {
			if err := dispatcher.Publish(ctx, "user.follow_requested", payload); err != nil {
				t.Fatal(err)
			}
		}
	}
	notificationCount := func() int {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND type_name='follow_request'`, owner).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	check("POST", path+"/follow-requests", "follower", "", 202)
	state("pending")
	dispatchLatestRequest()
	if notificationCount() != 1 || bot.calls != 1 {
		t.Fatalf("notifications=%d bot=%d", notificationCount(), bot.calls)
	}
	inbox := check("GET", "/notifications", "owner", "", 200)
	if !strings.Contains(inbox.Body.String(), `"type":"follow_request"`) || !strings.Contains(inbox.Body.String(), follower.String()) {
		t.Fatal(inbox.Body.String())
	}
	inbox = check("GET", "/notifications", "follower", "", 200)
	if strings.Contains(inbox.Body.String(), `"type":"follow_request"`) {
		t.Fatal("request notification leaked to sender")
	}
	prefs := check("GET", "/notification-settings", "owner", "", 200)
	if !strings.Contains(prefs.Body.String(), `"name":"follow_request"`) {
		t.Fatal("notification preference missing")
	}

	check("PUT", "/follows/"+owner.String(), "follower", "", 409)
	var requestedEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_messages WHERE event_name='user.follow_requested'`).Scan(&requestedEvents); err != nil || requestedEvents != 1 {
		t.Fatalf("duplicate request event: %d %v", requestedEvents, err)
	}

	for _, suffix := range privatePaths {
		check("GET", path+suffix, "follower", "", 404)
	}
	var events int
	countEvents := func() int {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_messages WHERE event_name='user.followed'`).Scan(&events); err != nil {
			t.Fatal(err)
		}
		return events
	}
	if countEvents() != 0 {
		t.Fatal("pending request published a follow event")
	}
	rec = check("GET", "/profile/follow-requests", "owner", "", 200)
	if !strings.Contains(rec.Body.String(), follower.String()) {
		t.Fatal("request not listed")
	}
	rec = check("GET", "/profile/follow-requests", "stranger", "", 200)
	if strings.Contains(rec.Body.String(), follower.String()) {
		t.Fatal("request leaked")
	}
	check("PUT", "/profile/follow-requests/"+follower.String(), "stranger", "", 404)
	check("DELETE", "/follows/"+owner.String(), "follower", "", 204)
	state("none")
	check("PUT", "/follows/"+owner.String(), "follower", "", 202)
	check("PATCH", "/notification-settings", "owner", `{"items":[{"name":"follow_request","enabled":false}]}`, 200)
	dispatchLatestRequest()
	if notificationCount() != 1 || bot.calls != 1 {
		t.Fatal("disabled request notification was sent")
	}

	check("DELETE", "/profile/follow-requests/"+follower.String(), "owner", "", 204)
	state("none")
	check("POST", path+"/follow-requests", "follower", "", 202)
	check("PATCH", "/notification-settings", "owner", `{"items":[{"name":"follow_request","enabled":true}]}`, 200)
	check("PATCH", "/settings", "owner", `{"allowNotifications":false}`, 200)
	dispatchLatestRequest()
	if notificationCount() != 2 || bot.calls != 1 {
		t.Fatal("global allowNotifications must suppress bot only")
	}
	// An outbox failure must roll back both approval and request deletion.
	if _, err := pool.Exec(ctx, `ALTER TABLE outbox_messages ADD CONSTRAINT test_follow_event_failure CHECK (event_name <> 'user.followed')`); err != nil {
		t.Fatal(err)
	}
	check("PUT", "/profile/follow-requests/"+follower.String(), "owner", "", 500)
	state("pending")
	if _, err := pool.Exec(ctx, `ALTER TABLE outbox_messages DROP CONSTRAINT test_follow_event_failure`); err != nil {
		t.Fatal(err)
	}
	// Concurrent approvals create exactly one follow and one event.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- request("PUT", "/profile/follow-requests/"+follower.String(), "owner", "").Code
		}()
	}
	wg.Wait()
	close(codes)
	var ok, missing int
	for code := range codes {
		if code == 204 {
			ok++
		} else if code == 404 {
			missing++
		} else {
			t.Fatalf("approval status %d", code)
		}
	}
	if ok != 1 || missing != 1 || countEvents() != 1 {
		t.Fatal("duplicate approval/event")
	}
	state("following")
	for _, suffix := range privatePaths {
		check("GET", path+suffix, "follower", "", 200)
		check("GET", path+suffix, "stranger", "", 404)
	}
	// Publicly published reviews remain readable even from a private profile.
	media, review, draft := uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO media_objects(id,title,type,info) VALUES($1,'Movie','movie','{"releaseStatus":"completed"}')`, media)
	exec(`INSERT INTO reviews(id,author_id,media_object_id,title,rating,status) VALUES($1,$2,$3,'Published',8,'published'),($4,$2,$3,'Draft',8,'draft')`, review, owner, media, draft)
	check("GET", "/reviews/"+review.String(), "", "", 200)
	check("GET", "/reviews/"+draft.String(), "", "", 404)
	check("GET", "/reviews/"+draft.String(), "owner", "", 200)
	rec = check("GET", "/feed", "", "", 200)
	if !strings.Contains(rec.Body.String(), review.String()) || strings.Contains(rec.Body.String(), draft.String()) {
		t.Fatal("feed visibility incorrect")
	}
	for _, target := range []struct{ method, path string }{{"GET", "/profile"}, {"PATCH", "/settings"}, {"GET", "/profile/follow-requests"}, {"POST", path + "/follow-requests"}, {"POST", "/reviews"}, {"PUT", "/follows/" + owner.String()}} {
		check(target.method, target.path, "", "", 401)
	}
	check("DELETE", "/follows/"+owner.String(), "follower", "", 204)
	state("none")
	check("GET", path, "follower", "", 404)
	check("PATCH", "/settings", "owner", `{"isProfilePrivate":false}`, 200)
	rec = check("PUT", "/follows/"+owner.String(), "follower", "", 200)
	state("following")
	var follow map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &follow); err != nil || follow["isFollowing"] != true {
		t.Fatalf("public follow=%s", rec.Body.String())
	}
	if countEvents() != 2 {
		t.Fatal("public follow event missing")
	}
}
