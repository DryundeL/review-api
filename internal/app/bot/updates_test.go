package bot

import (
	"strings"
	"testing"

	"review-api/internal/modules/users/contract"
)

func TestStartProfile(t *testing.T) {
	t.Parallel()
	profile, chatID, ok := startProfile(Update{
		UpdateID: 3,
		Message: &Message{
			From: &User{
				ID:           42,
				FirstName:    "Анна",
				LastName:     "Иванова",
				Username:     "anna",
				LanguageCode: "ru",
				IsPremium:    true,
			},
			Chat: Chat{ID: 42, Type: "private"},
			Text: "/start ref",
		},
	})
	if !ok || chatID != 42 {
		t.Fatalf("ok=%v chat=%d", ok, chatID)
	}
	want := contract.TelegramProfile{
		TelegramID: 42, FirstName: "Анна", LastName: "Иванова",
		Username: "anna", LanguageCode: "ru", IsPremium: true,
	}
	if profile != want {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestStartProfileIgnoresOtherMessages(t *testing.T) {
	t.Parallel()
	cases := []Update{
		{Message: &Message{Text: "/help", From: &User{ID: 1, FirstName: "A"}, Chat: Chat{ID: 1, Type: "private"}}},
		{Message: &Message{Text: "hello /start", From: &User{ID: 1, FirstName: "A"}, Chat: Chat{ID: 1, Type: "private"}}},
		{Message: &Message{Text: "/start"}},
		{},
	}
	for _, update := range cases {
		if _, _, ok := startProfile(update); ok {
			t.Fatalf("accepted %+v", update)
		}
	}
}

func TestStartCommandMentionAndGroupReply(t *testing.T) {
	t.Parallel()
	if !isStartCommand("/start@ReviewBot payload") {
		t.Fatal("expected start mention")
	}
	_, chatID, ok := startProfile(Update{
		Message: &Message{
			From: &User{ID: 9, FirstName: "Ann"},
			Chat: Chat{ID: -100, Type: "group"},
			Text: "/start@ReviewBot",
		},
	})
	if !ok || chatID != 0 {
		t.Fatalf("ok=%v chat=%d", ok, chatID)
	}
}

func TestStartMessageKeepsMiniAppButton(t *testing.T) {
	t.Parallel()
	const appURL = "https://app.example.com/review"
	text, markup := startMessage(contract.TelegramProfile{
		TelegramID:   42,
		FirstName:    "Анна\n",
		LastName:     "Иванова",
		Username:     "anna",
		LanguageCode: "ru",
		IsPremium:    true,
	}, appURL)
	for _, part := range []string{"Привет, Анна Иванова (@anna).", "Telegram ID: 42", "Язык: ru", "Premium: да", "Твои возможности ниже."} {
		if !strings.Contains(text, part) {
			t.Fatalf("text %q does not contain %q", text, part)
		}
	}
	if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("markup = %+v", markup)
	}
	button := markup.InlineKeyboard[0][0]
	if button.Text != "Открыть мини-приложение" || button.WebApp.URL != appURL {
		t.Fatalf("button = %+v", button)
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
			t.Fatalf("accepted %q", raw)
		}
	}
}
