package bot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authcontract "review-api/internal/modules/auth/contract"
)

type fakeLogins struct {
	login     authcontract.Login
	err       error
	confirmed int64
	denied    string
}

func (f *fakeLogins) GetLogin(context.Context, string) (authcontract.Login, error) {
	return f.login, f.err
}

func (f *fakeLogins) ConfirmLogin(_ context.Context, _ string, telegramID int64) error {
	f.confirmed = telegramID
	return f.err
}

func (f *fakeLogins) DenyLogin(_ context.Context, id string) error {
	f.denied = id
	return nil
}

func TestLoginPromptNamesBrowser(t *testing.T) {
	t.Parallel()
	const id = "0123456789abcdef0123456789abcdef"
	text, markup := loginPrompt("Chrome", "macOS", "ru", id)
	if !strings.Contains(text, "Браузер: Chrome") || !strings.Contains(text, "Устройство: macOS") {
		t.Fatalf("text = %q", text)
	}
	if markup.InlineKeyboard[0][0].Data != "y:"+id || markup.InlineKeyboard[0][1].Text != "Отклонить" {
		t.Fatalf("markup = %+v", markup)
	}
}

func TestHandleLoginConfirm(t *testing.T) {
	t.Parallel()
	const id = "0123456789abcdef0123456789abcdef"
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies[r.URL.Path] = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}))
	t.Cleanup(srv.Close)

	users := &fakeUsers{}
	logins := &fakeLogins{login: authcontract.Login{Browser: "Chrome", Device: "macOS", Status: authcontract.LoginPending}}
	client := NewClient("123456:abcDEF_token-1")
	client.base = srv.URL
	app := &App{
		users:  users,
		logins: logins,
		client: client,
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := app.handle(context.Background(), Update{
		Message: &Message{
			From: &User{ID: 42, FirstName: "Ann", LanguageCode: "ru"},
			Chat: Chat{ID: 42, Type: "private"},
			Text: "/start login_" + id,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sent := bodies["/bot123456:abcDEF_token-1/sendMessage"]
	text, _ := sent["text"].(string)
	if !strings.Contains(text, "Браузер: Chrome") || !strings.Contains(text, "Устройство: macOS") {
		t.Fatalf("prompt = %q", text)
	}

	err = app.handle(context.Background(), Update{
		CallbackQuery: &CallbackQuery{
			ID:   "cb",
			From: &User{ID: 42, FirstName: "Ann", LanguageCode: "ru"},
			Message: &Message{
				MessageID: 7,
				Chat:      Chat{ID: 42, Type: "private"},
			},
			Data: "y:" + id,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if users.calls != 1 || users.profile.TelegramID != 42 || logins.confirmed != 42 {
		t.Fatalf("users=%+v confirmed=%d", users.profile, logins.confirmed)
	}
	edited := bodies["/bot123456:abcDEF_token-1/editMessageText"]
	editedText, _ := edited["text"].(string)
	if !strings.Contains(editedText, "Вход подтверждён") {
		t.Fatalf("edited = %q", editedText)
	}
}

func TestHandleLoginDenyDoesNotCreateUser(t *testing.T) {
	t.Parallel()
	const id = "0123456789abcdef0123456789abcdef"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}))
	t.Cleanup(srv.Close)
	users := &fakeUsers{}
	logins := &fakeLogins{}
	client := NewClient("123456:abcDEF_token-1")
	client.base = srv.URL
	app := &App{users: users, logins: logins, client: client, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := app.handle(context.Background(), Update{
		CallbackQuery: &CallbackQuery{
			ID:      "cb",
			From:    &User{ID: 7, FirstName: "Ann"},
			Message: &Message{MessageID: 3, Chat: Chat{ID: 7, Type: "private"}},
			Data:    "n:" + id,
		},
	})
	if err != nil || users.calls != 0 || logins.denied != id {
		t.Fatalf("err=%v calls=%d denied=%s", err, users.calls, logins.denied)
	}
}
