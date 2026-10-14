package reviews

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/command"
	"review-api/internal/modules/reviews/internal/application/query"
	reviewshttp "review-api/internal/modules/reviews/internal/delivery/http"
	"review-api/internal/modules/reviews/internal/infrastructure/read"
	"review-api/internal/modules/reviews/internal/infrastructure/write"
)

type Dependencies struct {
	Pool  *pgxpool.Pool
	Admin echo.MiddlewareFunc
}

type Module struct {
	list   *query.ListTypesHandler
	create *command.CreateTypeHandler
	update *command.UpdateTypeHandler
	types  *read.Repository
	admin  echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	reader := read.NewRepository(deps.Pool)
	writer := write.NewRepository(deps.Pool)
	return &Module{
		list:   query.NewListTypesHandler(reader),
		create: command.NewCreateTypeHandler(writer),
		update: command.NewUpdateTypeHandler(writer),
		types:  reader,
		admin:  deps.Admin,
	}
}

func (m *Module) RequireActiveTypes(ctx context.Context, codes []string) error {
	if m == nil || m.types == nil {
		return contract.FieldError{Field: "type"}
	}
	return m.types.RequireActive(ctx, codes)
}

func (m *Module) statusCatalog() reviewshttp.StatusCatalog {
	return reviewshttp.StatusCatalog{
		Publication: statusItems(PublicationStatuses()),
		Release:     statusItems(ReleaseStatuses()),
	}
}

func statusItems(items []Status) []reviewshttp.StatusItem {
	out := make([]reviewshttp.StatusItem, 0, len(items))
	for _, item := range items {
		out = append(out, reviewshttp.StatusItem{Code: item.Code, Label: item.Label})
	}
	return out
}

func (m *Module) Name() string { return "reviews" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	reviewshttp.Register(g, reviewshttp.NewHandler(m.list, m.create, m.update, m.statusCatalog()), m.admin)
}
