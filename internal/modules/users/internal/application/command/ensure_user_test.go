package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
)

type fakeRepo struct {
	user     domain.User
	settings bool
	err      error
}

func (f *fakeRepo) Upsert(_ context.Context, user domain.User) error {
	if f.err != nil {
		return f.err
	}
	f.user = user
	return nil
}

func (f *fakeRepo) EnsureSettings(_ context.Context, telegramID int64, _ string) error {
	if f.err != nil {
		return f.err
	}
	if telegramID == f.user.TelegramID {
		f.settings = true
	}
	return nil
}

func (f *fakeRepo) AllowNotifications(_ context.Context, _ int64) (bool, error) {
	return true, nil
}

func (f *fakeRepo) AllowNotificationsByUserID(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakeRepo) SetAllowNotifications(_ context.Context, _ int64, _ bool) error {
	return nil
}

func (f *fakeRepo) GetSettings(_ context.Context, _ uuid.UUID) (domain.Settings, error) {
	return domain.Settings{}, nil
}

func (f *fakeRepo) UpdateSettings(_ context.Context, _ uuid.UUID, _ *string, _ *bool, _ *bool) (domain.Settings, error) {
	return domain.Settings{}, nil
}

func TestEnsureUserHandlerUpsertsProfile(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	now := time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)
	h := NewEnsureUserHandler(repo, func() time.Time { return now })

	err := h.Handle(context.Background(), contract.TelegramProfile{
		TelegramID:   42,
		FirstName:    "Ann",
		LastName:     "Lee",
		Username:     "ann",
		LanguageCode: "en",
		IsPremium:    true,
		PhotoURL:     "https://t.me/i/userpic/320/ann.jpg",
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if repo.user.TelegramID != 42 || repo.user.Username != "ann" || repo.user.FirstName != "Ann" || repo.user.LastName != "Lee" {
		t.Fatalf("stored = %+v", repo.user)
	}
	if !repo.user.IsPremium || repo.user.LanguageCode != "en" || !repo.user.UpdatedAt.Equal(now) {
		t.Fatalf("stored = %+v", repo.user)
	}
	if repo.user.PhotoURL != "https://t.me/i/userpic/320/ann.jpg" {
		t.Fatalf("photo = %q", repo.user.PhotoURL)
	}
	if !repo.settings {
		t.Fatal("expected settings ensure")
	}
}

func TestEnsureUserHandlerDoesNotWriteInvalidProfile(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	h := NewEnsureUserHandler(repo, time.Now)
	err := h.Handle(context.Background(), contract.TelegramProfile{TelegramID: 1, IsBot: true, FirstName: "Bot"})
	if !errors.Is(err, contract.ErrInvalidProfile) {
		t.Fatalf("err = %v", err)
	}
	if repo.user.TelegramID != 0 {
		t.Fatalf("wrote %+v", repo.user)
	}
}
