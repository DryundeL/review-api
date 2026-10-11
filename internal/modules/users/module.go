package users

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/command"
	"review-api/internal/modules/users/internal/infrastructure/write"
)

type Dependencies struct {
	Pool *pgxpool.Pool
}

type Module struct {
	ensure *command.EnsureUserHandler
}

func New(deps Dependencies) *Module {
	repo := write.NewRepository(deps.Pool)
	return &Module{ensure: command.NewEnsureUserHandler(repo, nil)}
}

func (m *Module) Name() string { return "users" }

func (m *Module) EnsureUser(ctx context.Context, profile contract.TelegramProfile) error {
	return m.ensure.Handle(ctx, profile)
}
