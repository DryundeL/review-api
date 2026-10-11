package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"review-api/internal/modules/users/contract"
)

type fakeUsers struct {
	profile contract.TelegramProfile
	err     error
	calls   int
}

func (f *fakeUsers) EnsureUser(_ context.Context, profile contract.TelegramProfile) error {
	f.calls++
	f.profile = profile
	return f.err
}

func TestHandleStartSavesProfileAndReplies(t *testing.T) {
	t.Parallel()
	var sawReply bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawReply = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	t.Cleanup(srv.Close)

	store := &fakeUsers{}
	client := NewClient("123456:abcDEF_token-1")
	client.base = srv.URL
	app := &App{users: store, client: client, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

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
	if !sawReply {
		t.Fatal("expected reply")
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
