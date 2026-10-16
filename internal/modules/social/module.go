package social

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/social/internal/application/command"
	"review-api/internal/modules/social/internal/application/query"
	socialhttp "review-api/internal/modules/social/internal/delivery/http"
	outboxpub "review-api/internal/modules/social/internal/infrastructure/outbox"
	"review-api/internal/modules/social/internal/infrastructure/read"
	"review-api/internal/modules/social/internal/infrastructure/write"
	"review-api/internal/platform/outbox"
	"review-api/internal/platform/transaction"
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
	follow        *command.FollowHandler
	unfollow      *command.UnfollowHandler
	listFollowing *query.ListFollowingHandler
	listFollowers *query.ListFollowersHandler
	users         UserResolver
	auth          echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	writer := write.NewRepository(deps.Pool)
	reader := read.NewRepository(deps.Pool)
	tx := transaction.NewPgxManager(deps.Pool)
	pub := outboxpub.New(outbox.NewStore(deps.Pool))
	return &Module{
		follow:        command.NewFollowHandler(writer, tx, pub),
		unfollow:      command.NewUnfollowHandler(writer),
		listFollowing: query.NewListFollowingHandler(reader),
		listFollowers: query.NewListFollowersHandler(reader),
		users:         deps.Users,
		auth:          deps.Auth,
	}
}

func (m *Module) Name() string { return "social" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	socialhttp.Register(g, socialhttp.NewHandler(
		m.follow, m.unfollow, m.listFollowing, m.listFollowers, m.users,
	), m.auth)
}
