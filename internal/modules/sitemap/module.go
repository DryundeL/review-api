package sitemap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/sitemap/internal/application/query"
	sitemaphttp "review-api/internal/modules/sitemap/internal/delivery/http"
	"review-api/internal/modules/sitemap/internal/infrastructure/read"
)

type Dependencies struct{ Pool *pgxpool.Pool }
type Module struct{ list *query.ListHandler }

func New(deps Dependencies) *Module {
	return &Module{list: query.NewListHandler(read.NewRepository(deps.Pool))}
}

func (m *Module) Name() string               { return "sitemap" }
func (m *Module) RegisterHTTP(g *echo.Group) { sitemaphttp.Register(g, m.list) }
