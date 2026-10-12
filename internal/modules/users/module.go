package users

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/command"
	"review-api/internal/modules/users/internal/application/query"
	usershttp "review-api/internal/modules/users/internal/delivery/http"
	"review-api/internal/modules/users/internal/infrastructure/read"
	"review-api/internal/modules/users/internal/infrastructure/write"
)

type Dependencies struct {
	Pool *pgxpool.Pool
	Auth echo.MiddlewareFunc
}

type Module struct {
	ensure  *command.EnsureUserHandler
	profile *query.GetProfileHandler
	auth    echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	return &Module{
		ensure:  command.NewEnsureUserHandler(write.NewRepository(deps.Pool), nil),
		profile: query.NewGetProfileHandler(read.NewRepository(deps.Pool)),
		auth:    deps.Auth,
	}
}

func (m *Module) Name() string { return "users" }

func (m *Module) EnsureUser(ctx context.Context, profile contract.TelegramProfile) error {
	return m.ensure.Handle(ctx, profile)
}

func (m *Module) RegisterHTTP(g *echo.Group) {
	usershttp.Register(g, usershttp.NewHandler(m.profile), m.auth)
}
