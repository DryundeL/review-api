package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
)

type settingsFakeRepo struct {
	fakeRepo
	stored domain.Settings
}

func (f *settingsFakeRepo) GetSettings(_ context.Context, userID uuid.UUID) (domain.Settings, error) {
	if f.stored.UserID == uuid.Nil {
		return domain.Settings{}, contract.ErrNotFound
	}
	f.stored.UserID = userID
	return f.stored, nil
}

func (f *settingsFakeRepo) UpdateSettings(_ context.Context, userID uuid.UUID, language *string, muted *bool) (domain.Settings, error) {
	f.stored.UserID = userID
	if language != nil {
		f.stored.Language = *language
	}
	if muted != nil {
		f.stored.NotificationsMuted = *muted
	}
	return f.stored, nil
}

func TestUpdateSettingsHandlerValidatesLanguage(t *testing.T) {
	t.Parallel()
	repo := &settingsFakeRepo{stored: domain.Settings{Language: "en"}}
	h := NewUpdateSettingsHandler(repo)
	bad := "!"
	_, err := h.Handle(context.Background(), uuid.New(), UpdateSettingsInput{Language: &bad})
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "language" {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateSettingsHandlerPatchesMute(t *testing.T) {
	t.Parallel()
	repo := &settingsFakeRepo{stored: domain.Settings{Language: "ru"}}
	h := NewUpdateSettingsHandler(repo)
	muted := true
	id := uuid.New()
	got, err := h.Handle(context.Background(), id, UpdateSettingsInput{NotificationsMuted: &muted})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !got.NotificationsMuted || got.Language != "ru" {
		t.Fatalf("got = %+v", got)
	}
}
