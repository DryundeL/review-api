package bot

import (
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

func TestStartReplyLanguage(t *testing.T) {
	t.Parallel()
	if startReply("ru") != "Профиль сохранён." || startReply("") != "Профиль сохранён." {
		t.Fatal("russian reply")
	}
	if startReply("en") != "Profile saved." {
		t.Fatal("english reply")
	}
}
