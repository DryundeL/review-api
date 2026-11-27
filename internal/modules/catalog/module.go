package catalog

import (
	"context"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/command"
	"review-api/internal/modules/catalog/internal/application/query"
	cataloghttp "review-api/internal/modules/catalog/internal/delivery/http"
	outboxpub "review-api/internal/modules/catalog/internal/infrastructure/outbox"
	"review-api/internal/modules/catalog/internal/infrastructure/read"
	"review-api/internal/modules/catalog/internal/infrastructure/write"
	"review-api/internal/platform/outbox"
	"review-api/internal/platform/transaction"
)

type TypeChecker interface {
	RequireActive(ctx context.Context, codes []string) error
}

type Dependencies struct {
	Pool  *pgxpool.Pool
	Admin echo.MiddlewareFunc
	Types TypeChecker
}

type Module struct {
	importBatch *command.ImportMediaObjectsHandler
	get         *query.GetMediaObjectHandler
	search      *query.SearchMediaObjectsHandler
	admin       echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	reader := read.NewRepository(deps.Pool)
	writer := write.NewRepository(deps.Pool)
	return &Module{
		importBatch: command.NewImportMediaObjectsHandler(
			writer,
			transaction.NewPgxManager(deps.Pool),
			typeBridge{deps.Types},
			outboxpub.New(outbox.NewStore(deps.Pool)),
		),
		get:    query.NewGetMediaObjectHandler(reader),
		search: query.NewSearchMediaObjectsHandler(reader),
		admin:  deps.Admin,
	}
}

type typeBridge struct{ next TypeChecker }

func (b typeBridge) RequireActive(ctx context.Context, codes []string) error {
	if b.next == nil {
		return contract.FieldError{Field: "type", Index: -1}
	}
	return b.next.RequireActive(ctx, codes)
}

func (m *Module) Name() string { return "catalog" }

// MediaObjectRef returns type, title and poster of a work; contract.ErrNotFound if it does not exist.
func (m *Module) MediaObjectRef(ctx context.Context, id uuid.UUID) (contract.MediaObjectRef, error) {
	if m == nil || m.get == nil {
		return contract.MediaObjectRef{}, contract.ErrNotFound
	}
	item, err := m.get.Handle(ctx, id)
	if err != nil {
		return contract.MediaObjectRef{}, err
	}
	return contract.MediaObjectRef{ID: item.ID, Type: item.Type, Title: item.Title, PosterURL: item.PosterURL}, nil
}

func (m *Module) RegisterHTTP(g *echo.Group) {
	cataloghttp.Register(g, cataloghttp.NewHandler(m.importBatch, m.get, m.search), m.admin)
}
