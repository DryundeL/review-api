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

type AuthorResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Dependencies struct {
	Pool    *pgxpool.Pool
	Auth    echo.MiddlewareFunc
	Types   TypeChecker
	Authors AuthorResolver
}

type Module struct {
	create *command.CreateWorkHandler
	get    *query.GetWorkHandler
	search *query.SearchWorksHandler
	auth   echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	reader := read.NewRepository(deps.Pool)
	writer := write.NewRepository(deps.Pool)
	return &Module{
		create: command.NewCreateWorkHandler(writer, transaction.NewPgxManager(deps.Pool), typeBridge{deps.Types}, authorBridge{deps.Authors}, outboxpub.New(outbox.NewStore(deps.Pool))),
		get:    query.NewGetWorkHandler(reader),
		search: query.NewSearchWorksHandler(reader),
		auth:   deps.Auth,
	}
}

type typeBridge struct{ next TypeChecker }

func (b typeBridge) RequireActive(ctx context.Context, codes []string) error {
	if b.next == nil {
		return contract.FieldError{Field: "type"}
	}
	return b.next.RequireActive(ctx, codes)
}

type authorBridge struct{ next AuthorResolver }

func (b authorBridge) UserID(ctx context.Context, telegramID int64) (uuid.UUID, error) {
	if b.next == nil {
		return uuid.Nil, contract.ErrAuthorNotFound
	}
	return b.next.UserID(ctx, telegramID)
}

func (m *Module) Name() string { return "catalog" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	cataloghttp.Register(g, cataloghttp.NewHandler(m.create, m.get, m.search), m.auth)
}
