package feed

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/feed/internal/application/query"
	feedhttp "review-api/internal/modules/feed/internal/delivery/http"
	"review-api/internal/modules/feed/internal/infrastructure/read"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Dependencies struct {
	Pool  *pgxpool.Pool
	Auth  echo.MiddlewareFunc
	Users UserResolver
}

type Module struct {
	list  *query.ListFeedHandler
	users UserResolver
	auth  echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	repo := read.NewRepository(deps.Pool)
	return &Module{
		list:  query.NewListFeedHandler(repo),
		users: deps.Users,
		auth:  deps.Auth,
	}
}

func (m *Module) Name() string { return "feed" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	feedhttp.Register(g, feedhttp.NewHandler(m.list, m.users), m.auth)
}
