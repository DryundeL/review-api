package command

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
	"testing"
)

type contentRepo struct {
	saved []string
	err   error
}

func (r *contentRepo) GetContentPreferences(context.Context, uuid.UUID) (domain.ContentPreferences, error) {
	return domain.ContentPreferences{Types: r.saved, NeedsOnboarding: len(r.saved) == 0}, r.err
}
func (r *contentRepo) SaveContentPreferences(_ context.Context, _ uuid.UUID, types []string) (domain.ContentPreferences, error) {
	r.saved = types
	return domain.ContentPreferences{Types: types}, r.err
}
func TestContentPreferencesLifecycle(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := &contentRepo{}
	h := NewContentPreferencesHandler(repo)
	before, err := h.Get(ctx, id)
	if err != nil || !before.NeedsOnboarding {
		t.Fatalf("before: %+v %v", before, err)
	}
	saved, err := h.Save(ctx, id, []string{"manga", "book"})
	if err != nil || saved.NeedsOnboarding || len(saved.Types) != 2 {
		t.Fatalf("save: %+v %v", saved, err)
	}
	after, err := h.Get(ctx, id)
	if err != nil || after.NeedsOnboarding {
		t.Fatalf("after: %+v %v", after, err)
	}
	if _, err := h.Save(ctx, id, []string{}); err == nil || len(repo.saved) != 2 {
		t.Fatal("invalid request changed preferences")
	}
	if _, err := h.Save(ctx, uuid.Nil, []string{"manga"}); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("nil user: %v", err)
	}
	repo.err = errors.New("storage unavailable")
	if _, err := h.Get(ctx, id); !errors.Is(err, repo.err) {
		t.Fatalf("lost error: %v", err)
	}
}
