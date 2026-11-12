package command

import (
	"context"
	"github.com/google/uuid"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
)

type ContentPreferencesRepository interface {
	GetContentPreferences(context.Context, uuid.UUID) (domain.ContentPreferences, error)
	SaveContentPreferences(context.Context, uuid.UUID, []string) (domain.ContentPreferences, error)
}

type ContentPreferencesHandler struct{ repo ContentPreferencesRepository }

func NewContentPreferencesHandler(repo ContentPreferencesRepository) *ContentPreferencesHandler {
	return &ContentPreferencesHandler{repo: repo}
}
func (h *ContentPreferencesHandler) Get(ctx context.Context, id uuid.UUID) (domain.ContentPreferences, error) {
	if id == uuid.Nil {
		return domain.ContentPreferences{}, contract.ErrNotFound
	}
	return h.repo.GetContentPreferences(ctx, id)
}
func (h *ContentPreferencesHandler) Save(ctx context.Context, id uuid.UUID, types []string) (domain.ContentPreferences, error) {
	if id == uuid.Nil {
		return domain.ContentPreferences{}, contract.ErrNotFound
	}
	clean, err := domain.ContentTypes(types)
	if err != nil {
		return domain.ContentPreferences{}, err
	}
	return h.repo.SaveContentPreferences(ctx, id, clean)
}
