package search

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/search/internal/application/query"
	searchhttp "review-api/internal/modules/search/internal/delivery/http"
	"review-api/internal/modules/search/internal/infrastructure/read"
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
	trends *query.TrendsHandler
	search *query.SearchHandler
	users  UserResolver
	auth   echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	repo := read.NewRepository(deps.Pool)
	return &Module{
		trends: query.NewTrendsHandler(repo),
		search: query.NewSearchHandler(repo),
		users:  deps.Users,
		auth:   deps.Auth,
	}
}

func (m *Module) Name() string { return "search" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	searchhttp.Register(g, searchhttp.NewHandler(m.trends, m.search, m.users), m.auth)
}
