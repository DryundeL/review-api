package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/contract"
	"review-api/internal/modules/notifications/internal/application/ports"
)

type fakePrefs struct {
	like          bool
	follow        bool
	followRequest bool
	ensured       bool
}

func (f *fakePrefs) EnsureDefaults(_ context.Context, _ uuid.UUID) error {
	f.ensured = true
	return nil
}

func (f *fakePrefs) List(_ context.Context, _ uuid.UUID) ([]ports.PrefItem, error) {
	return []ports.PrefItem{
		{Name: "follow", Title: "follow", Text: "follow", Enabled: f.follow},
		{Name: "follow_request", Title: "follow_request", Text: "follow_request", Enabled: f.followRequest},
		{Name: "like", Title: "like", Text: "like", Enabled: f.like},
	}, nil
}

func (f *fakePrefs) Patch(_ context.Context, _ uuid.UUID, like, follow, followRequest *bool) error {
	if like != nil {
		f.like = *like
	}
	if follow != nil {
		f.follow = *follow
	}
	if followRequest != nil {
		f.followRequest = *followRequest
	}
	return nil
}

func (f *fakePrefs) TypeEnabled(_ context.Context, _ uuid.UUID, name string) (bool, error) {
	switch name {
	case "like":
		return f.like, nil
	case "follow":
		return f.follow, nil
	case "follow_request":
		return f.followRequest, nil
	default:
		return false, nil
	}
}

func (f *fakePrefs) TypeMeta(_ context.Context, name string) (ports.TypeMeta, error) {
	return ports.TypeMeta{Name: name, Title: name, Text: name}, nil
}

func TestUpdatePrefsRejectsUnknownType(t *testing.T) {
	t.Parallel()
	h := NewUpdatePrefsHandler(&fakePrefs{like: true, follow: true})
	_, err := h.Handle(context.Background(), uuid.New(), []ports.PrefPatch{
		{Name: "comment", Enabled: false},
	})
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "name" {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdatePrefsPatchesKnownType(t *testing.T) {
	t.Parallel()
	repo := &fakePrefs{like: true, follow: true}
	h := NewUpdatePrefsHandler(repo)
	_, err := h.Handle(context.Background(), uuid.New(), []ports.PrefPatch{
		{Name: "like", Enabled: false},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if repo.like || !repo.follow || !repo.ensured {
		t.Fatalf("repo = %+v", repo)
	}
}

func TestUpdateFollowRequestPreference(t *testing.T) {
	repo := &fakePrefs{like: true, follow: true, followRequest: true}
	_, err := NewUpdatePrefsHandler(repo).Handle(context.Background(), uuid.New(), []ports.PrefPatch{{Name: "follow_request", Enabled: false}})
	if err != nil || repo.followRequest || !repo.follow || !repo.like {
		t.Fatalf("prefs=%+v err=%v", repo, err)
	}
}
