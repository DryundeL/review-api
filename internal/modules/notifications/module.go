package notifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/notifications/internal/application/command"
	"review-api/internal/modules/notifications/internal/application/query"
	notificationshttp "review-api/internal/modules/notifications/internal/delivery/http"
	"review-api/internal/modules/notifications/internal/infrastructure/write"
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
	list   *query.ListPrefsHandler
	update *command.UpdatePrefsHandler
	users  UserResolver
	auth   echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	repo := write.NewRepository(deps.Pool)
	return &Module{
		list:   query.NewListPrefsHandler(repo),
		update: command.NewUpdatePrefsHandler(repo),
		users:  deps.Users,
		auth:   deps.Auth,
	}
}

func (m *Module) Name() string { return "notifications" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	notificationshttp.Register(g, notificationshttp.NewHandler(m.list, m.update, m.users), m.auth)
}
