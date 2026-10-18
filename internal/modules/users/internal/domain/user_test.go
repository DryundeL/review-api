package domain_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
)

func TestNewUserStoresProfileFields(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)
	user, err := domain.NewUser(contract.TelegramProfile{
		TelegramID:   42,
		FirstName:    "  Анна ",
		LastName:     "Иванова",
		Username:     "@Anna_1",
		LanguageCode: "ru",
		IsPremium:    true,
		PhotoURL:     " https://t.me/i/userpic/320/abc.jpg ",
	}, now)
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	if user.TelegramID != 42 || user.FirstName != "Анна" || user.LastName != "Иванова" {
		t.Fatalf("names = %+v", user)
	}
	if user.Username != "Anna_1" || user.LanguageCode != "ru" || !user.IsPremium {
		t.Fatalf("profile = %+v", user)
	}
	if user.PhotoURL != "https://t.me/i/userpic/320/abc.jpg" {
		t.Fatalf("photo = %q", user.PhotoURL)
	}
	if user.ID == uuid.Nil || !user.CreatedAt.Equal(now) || !user.UpdatedAt.Equal(now) {
		t.Fatalf("identity timestamps = %+v", user)
	}
}

func TestNewUserRejectsBotAndEmptyName(t *testing.T) {
	t.Parallel()
	cases := []contract.TelegramProfile{
		{TelegramID: 1, IsBot: true, FirstName: "Bot"},
		{TelegramID: 0, FirstName: "Ann"},
		{TelegramID: 1, FirstName: " \n\t "},
	}
	for _, p := range cases {
		if _, err := domain.NewUser(p, time.Time{}); err != contract.ErrInvalidProfile {
			t.Fatalf("profile %+v err = %v", p, err)
		}
	}
}

func TestNewUserDropsUnsafeOptionalFields(t *testing.T) {
	t.Parallel()
	user, err := domain.NewUser(contract.TelegramProfile{
		TelegramID:   7,
		FirstName:    "Ann" + "\u202e" + strings.Repeat("я", 80),
		LastName:     "Bad name\n",
		Username:     "not a user",
		LanguageCode: "ru;drop",
		PhotoURL:     "http://evil.example/x",
	}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	if strings.Contains(user.FirstName, "\u202e") || utf8.RuneCountInString(user.FirstName) > 64 {
		t.Fatalf("first name = %q", user.FirstName)
	}
	if user.LastName != "Bad name" || user.Username != "" || user.LanguageCode != "" || user.PhotoURL != "" {
		t.Fatalf("optional = %+v", user)
	}
}
