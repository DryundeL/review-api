package notifications

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/notifications/internal/application/command"
	"review-api/internal/modules/notifications/internal/application/consumer"
	"review-api/internal/modules/notifications/internal/application/query"
	notificationshttp "review-api/internal/modules/notifications/internal/delivery/http"
	"review-api/internal/modules/notifications/internal/infrastructure/write"
	reviewscontract "review-api/internal/modules/reviews/contract"
	socialcontract "review-api/internal/modules/social/contract"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/eventbus"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
	UserRef(ctx context.Context, userID uuid.UUID) (userscontract.UserRef, error)
	AllowNotificationsByUserID(ctx context.Context, userID uuid.UUID) (bool, error)
}

type ReviewLookup interface {
	PublishedReviewRef(ctx context.Context, reviewID uuid.UUID) (reviewscontract.ReviewRef, error)
}

type BotSender interface {
	SendMessage(ctx context.Context, chatID int64, text string, markup any) error
}

type Dependencies struct {
	Pool    *pgxpool.Pool
	Auth    echo.MiddlewareFunc
	Users   UserResolver
	Reviews ReviewLookup
	Bot     BotSender
	Log     *slog.Logger
}

type Module struct {
	listPrefs       *query.ListPrefsHandler
	updatePrefs     *command.UpdatePrefsHandler
	listInbox       *query.ListInboxHandler
	unread          *query.UnreadCountHandler
	markRead        *command.MarkReadHandler
	markAllRead     *command.MarkAllReadHandler
	voted           *consumer.VotedHandler
	followed        *consumer.FollowedHandler
	followRequested *consumer.FollowRequestedHandler
	users           UserResolver
	auth            echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	repo := write.NewRepository(deps.Pool)
	create := command.NewCreateNotificationHandler(repo, repo, deps.Users, deps.Users, deps.Bot, deps.Log)
	return &Module{
		listPrefs:       query.NewListPrefsHandler(repo),
		updatePrefs:     command.NewUpdatePrefsHandler(repo),
		listInbox:       query.NewListInboxHandler(repo),
		unread:          query.NewUnreadCountHandler(repo),
		markRead:        command.NewMarkReadHandler(repo),
		markAllRead:     command.NewMarkAllReadHandler(repo),
		voted:           consumer.NewVotedHandler(create, deps.Reviews),
		followed:        consumer.NewFollowedHandler(create),
		followRequested: consumer.NewFollowRequestedHandler(create),
		users:           deps.Users,
		auth:            deps.Auth,
	}
}

func (m *Module) Name() string { return "notifications" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	notificationshttp.Register(g, notificationshttp.NewHandler(
		m.listPrefs, m.updatePrefs, m.listInbox, m.unread, m.markRead, m.markAllRead, m.users,
	), m.auth)
}

func (m *Module) RegisterConsumers(d *eventbus.Dispatcher) {
	if m == nil || d == nil {
		return
	}
	if m.voted != nil {
		d.Subscribe(reviewscontract.EventVoted, m.voted.Handle)
	}
	if m.followRequested != nil {
		d.Subscribe(socialcontract.EventFollowRequested, m.followRequested.Handle)
	}
	if m.followed != nil {
		d.Subscribe(socialcontract.EventFollowed, m.followed.Handle)
	}
}
