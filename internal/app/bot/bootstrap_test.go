package bot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"review-api/internal/modules/users/contract"
)

type fakeUsers struct {
	profile contract.TelegramProfile
	err     error
	calls   int
	allow   bool
}

func (f *fakeUsers) EnsureUser(_ context.Context, profile contract.TelegramProfile) error {
	f.calls++
	f.profile = profile
	return f.err
}

func (f *fakeUsers) AllowNotifications(_ context.Context, _ int64) (bool, error) {
	return f.allow, nil
}

func (f *fakeUsers) SetAllowNotifications(_ context.Context, _ int64, allow bool) error {
	f.allow = allow
	return nil
}

func TestHandleStartSavesProfileAndReplies(t *testing.T) {
	t.Parallel()
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		bodies[r.URL.Path] = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}))
	t.Cleanup(srv.Close)

	store := &fakeUsers{allow: true}
	client := NewClient("123456:abcDEF_token-1")
	client.base = srv.URL
	app := &App{
		users:       store,
		client:      client,
		log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		miniAppURL:  "https://app.example.com",
		miniAppName: "Review Hub",
	}

	err := app.handle(context.Background(), Update{
		UpdateID: 1,
		Message: &Message{
			From: &User{ID: 42, FirstName: "Ann", LastName: "Lee", Username: "ann", LanguageCode: "en", IsPremium: true},
			Chat: Chat{ID: 42, Type: "private"},
			Text: "/start",
		},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if store.calls != 1 || store.profile.TelegramID != 42 || store.profile.Username != "ann" || !store.profile.IsPremium {
		t.Fatalf("saved %+v calls=%d", store.profile, store.calls)
	}
	menu := bodies["/bot123456:abcDEF_token-1/setChatMenuButton"]
	menuButton, _ := menu["menu_button"].(map[string]any)
	if menuButton["type"] != "web_app" || menuButton["text"] != "Review Hub" {
		t.Fatalf("menu button = %#v", menu["menu_button"])
	}
	message := bodies["/bot123456:abcDEF_token-1/sendMessage"]
	text, _ := message["text"].(string)
	if !strings.Contains(text, "Hi, Ann Lee (@ann).") || !strings.Contains(text, "Notifications: on.") {
		t.Fatalf("text = %q", text)
	}
	markup, _ := message["reply_markup"].(map[string]any)
	rows, _ := markup["keyboard"].([]any)
	if len(rows) != 1 || markup["is_persistent"] != true {
		t.Fatalf("markup = %#v", message["reply_markup"])
	}
	row, _ := rows[0].([]any)
	if len(row) != 2 {
		t.Fatalf("keyboard row = %#v", rows[0])
	}
	startBtn, _ := row[0].(map[string]any)
	muteBtn, _ := row[1].(map[string]any)
	if startBtn["text"] != "Start" || muteBtn["text"] != "Mute notifications" {
		t.Fatalf("buttons = %#v %#v", startBtn, muteBtn)
	}
	if _, hasWebApp := startBtn["web_app"]; hasWebApp {
		t.Fatal("expected no web_app on command keyboard")
	}
}

func TestHandleMuteUpdatesKeyboard(t *testing.T) {
	t.Parallel()
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			last = body
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}))
	t.Cleanup(srv.Close)

	store := &fakeUsers{allow: true}
	client := NewClient("123456:abcDEF_token-1")
	client.base = srv.URL
	app := &App{
		users:       store,
		client:      client,
		log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		miniAppURL:  "https://app.example.com",
		miniAppName: "Review Hub",
	}
	err := app.handle(context.Background(), Update{
		Message: &Message{
			From: &User{ID: 42, FirstName: "Ann", LanguageCode: "ru"},
			Chat: Chat{ID: 42, Type: "private"},
			Text: "/mute",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.allow {
		t.Fatal("expected allow=false after mute")
	}
	text, _ := last["text"].(string)
	if !strings.Contains(text, "Уведомления выключены") {
		t.Fatalf("text = %q", text)
	}
	markup, _ := last["reply_markup"].(map[string]any)
	rows, _ := markup["keyboard"].([]any)
	row, _ := rows[0].([]any)
	unmute, _ := row[1].(map[string]any)
	if unmute["text"] != "Включить уведомления" {
		t.Fatalf("keyboard = %#v", markup)
	}
}

func TestHandleStartSkipsInvalidProfile(t *testing.T) {
	t.Parallel()
	store := &fakeUsers{err: contract.ErrInvalidProfile}
	app := &App{users: store, client: NewClient("123456:abcDEF_token-1"), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := app.handle(context.Background(), Update{
		Message: &Message{
			From: &User{ID: 1, IsBot: true, FirstName: "Bot"},
			Chat: Chat{ID: 1, Type: "private"},
			Text: "/start",
		},
	})
	if err != nil || store.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, store.calls)
	}
}

func TestHandleStartRetriesDatabaseError(t *testing.T) {
	t.Parallel()
	store := &fakeUsers{err: errors.New("db down")}
	app := &App{users: store, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := app.handle(context.Background(), Update{
		Message: &Message{
			From: &User{ID: 1, FirstName: "Ann"},
			Chat: Chat{ID: 1, Type: "private"},
			Text: "/start",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
