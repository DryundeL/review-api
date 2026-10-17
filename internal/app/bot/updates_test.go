package bot

import (
	"strings"
	"testing"

	"review-api/internal/modules/users/contract"
)

func TestStartProfileRequiresStartCommand(t *testing.T) {
	t.Parallel()
	okUpdate := Update{
		Message: &Message{
			From: &User{ID: 1, FirstName: "Ann", LanguageCode: "en"},
			Chat: Chat{ID: 1, Type: "private"},
			Text: "/start ref",
		},
	}
	profile, chatID, ok := startProfile(okUpdate)
	if !ok || profile.TelegramID != 1 || chatID != 1 || profile.FirstName != "Ann" {
		t.Fatalf("got ok=%v profile=%+v chat=%d", ok, profile, chatID)
	}

	for _, update := range []Update{
		{},
		{Message: &Message{Text: "/start", Chat: Chat{ID: 1, Type: "private"}}},
		{Message: &Message{Text: "hello /start", From: &User{ID: 1, FirstName: "A"}, Chat: Chat{ID: 1, Type: "private"}}},
		{Message: &Message{Text: "/start"}},
	} {
		if _, _, ok := startProfile(update); ok {
			t.Fatalf("expected reject for %#v", update.Message)
		}
	}
}

func TestStartCommandMentionAndGroupReply(t *testing.T) {
	t.Parallel()
	if !isStartCommand("/start@ReviewBot payload") {
		t.Fatal("expected start command with mention")
	}
	profile, chatID, ok := startProfile(Update{
		Message: &Message{
			From: &User{ID: 7, FirstName: "Ann"},
			Chat: Chat{ID: -100, Type: "supergroup"},
			Text: "/start@ReviewBot",
		},
	})
	if !ok || profile.TelegramID != 7 || chatID != 0 {
		t.Fatalf("got ok=%v profile=%+v chat=%d", ok, profile, chatID)
	}
}

func TestStartMessageUsesCommandKeyboard(t *testing.T) {
	t.Parallel()
	view := startMessage(contract.TelegramProfile{
		TelegramID:   42,
		FirstName:    "Анна\n",
		LastName:     "Иванова",
		Username:     "anna",
		LanguageCode: "ru",
		IsPremium:    true,
	}, "Review Hub", false)
	for _, part := range []string{"Привет, Анна Иванова (@anna).", "Telegram ID: 42", "Язык: ru", "Premium: да", "Уведомления: включены."} {
		if !strings.Contains(view.Text, part) {
			t.Fatalf("text %q does not contain %q", view.Text, part)
		}
	}
	if view.MenuButton != "Review Hub" {
		t.Fatalf("menu button = %q", view.MenuButton)
	}
	if !view.Markup.ResizeKeyboard || !view.Markup.IsPersistent {
		t.Fatalf("markup flags = %+v", view.Markup)
	}
	if len(view.Markup.Keyboard) != 1 || len(view.Markup.Keyboard[0]) != 2 {
		t.Fatalf("markup = %+v", view.Markup)
	}
	if view.Markup.Keyboard[0][0].Text != "Старт" || view.Markup.Keyboard[0][1].Text != "Выключить уведомления" {
		t.Fatalf("buttons = %+v", view.Markup.Keyboard[0])
	}

	muted := startMessage(contract.TelegramProfile{TelegramID: 1, FirstName: "Ann", LanguageCode: "en"}, "Review Hub", true)
	if muted.Markup.Keyboard[0][1].Text != "Unmute notifications" {
		t.Fatalf("muted button = %+v", muted.Markup.Keyboard[0])
	}
	if !strings.Contains(muted.Text, "Notifications: muted.") {
		t.Fatalf("text = %q", muted.Text)
	}
}

func TestClassifyMessageButtonsAndCommands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text, lang string
		want       botAction
	}{
		{"/start", "en", actionStart},
		{"/mute@bot", "ru", actionMute},
		{"/unmute", "en", actionUnmute},
		{"Старт", "ru", actionStart},
		{"Выключить уведомления", "ru", actionMute},
		{"Включить уведомления", "ru", actionUnmute},
		{"Start", "en", actionStart},
		{"Mute notifications", "en", actionMute},
		{"Unmute notifications", "en", actionUnmute},
		{"hello", "ru", actionNone},
	}
	for _, tc := range cases {
		if got := classifyMessage(tc.text, tc.lang); got != tc.want {
			t.Fatalf("%q/%q = %v want %v", tc.text, tc.lang, got, tc.want)
		}
	}
}

func TestParseMiniAppName(t *testing.T) {
	t.Parallel()
	got, err := parseMiniAppName(" Review Hub ")
	if err != nil || got != "Review Hub" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, raw := range []string{"", "   ", "This name is way too long"} {
		if _, err := parseMiniAppName(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestParseMiniAppURL(t *testing.T) {
	t.Parallel()
	got, err := parseMiniAppURL(" https://app.example.com/review ")
	if err != nil || got != "https://app.example.com/review" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, raw := range []string{"", "http://app.example.com", "https://user:pass@app.example.com", "https://localhost/app"} {
		if _, err := parseMiniAppURL(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}
