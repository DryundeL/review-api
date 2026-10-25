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

func (f *settingsFakeRepo) UpdateSettings(_ context.Context, userID uuid.UUID, language *string, allow *bool, private *bool) (domain.Settings, error) {
	f.stored.UserID = userID
	if language != nil {
		f.stored.Language = *language
	}
	if allow != nil {
		f.stored.AllowNotifications = *allow
	}
	if private != nil {
		f.stored.IsProfilePrivate = *private
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

func TestUpdateSettingsHandlerPatchesAllow(t *testing.T) {
	t.Parallel()
	repo := &settingsFakeRepo{stored: domain.Settings{Language: "ru", AllowNotifications: true}}
	h := NewUpdateSettingsHandler(repo)
	allow := false
	id := uuid.New()
	got, err := h.Handle(context.Background(), id, UpdateSettingsInput{AllowNotifications: &allow})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got.AllowNotifications || got.Language != "ru" {
		t.Fatalf("got = %+v", got)
	}
}

func TestUpdateSettingsPrivacy(t *testing.T) {
	id := uuid.New()
	repo := &settingsFakeRepo{stored: domain.Settings{UserID: id, Language: "ru", AllowNotifications: true}}
	h := NewUpdateSettingsHandler(repo)
	for _, private := range []bool{true, false} {
		got, err := h.Handle(context.Background(), id, UpdateSettingsInput{IsProfilePrivate: &private})
		if err != nil || got.IsProfilePrivate != private || got.Language != "ru" || !got.AllowNotifications {
			t.Fatalf("settings=%+v err=%v", got, err)
		}
	}
}
