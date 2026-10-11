package command

import (
	"context"
	"fmt"
	"time"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

type EnsureUserHandler struct {
	repo ports.UserRepository
	now  func() time.Time
}

func NewEnsureUserHandler(repo ports.UserRepository, now func() time.Time) *EnsureUserHandler {
	if now == nil {
		now = time.Now
	}
	return &EnsureUserHandler{repo: repo, now: now}
}

func (h *EnsureUserHandler) Handle(ctx context.Context, profile contract.TelegramProfile) error {
	user, err := domain.NewUser(profile, h.now())
	if err != nil {
		return err
	}
	if err := h.repo.Upsert(ctx, user); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}
