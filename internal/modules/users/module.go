package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/command"
	"review-api/internal/modules/users/internal/application/ports"
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
	ensure         *command.EnsureUserHandler
	profile        *query.GetProfileHandler
	getSettings    *query.GetSettingsHandler
	updateSettings *command.UpdateSettingsHandler
	prefs          ports.UserRepository
	auth           echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	writeRepo := write.NewRepository(deps.Pool)
	return &Module{
		ensure:         command.NewEnsureUserHandler(writeRepo, nil),
		profile:        query.NewGetProfileHandler(read.NewRepository(deps.Pool)),
		getSettings:    query.NewGetSettingsHandler(writeRepo),
		updateSettings: command.NewUpdateSettingsHandler(writeRepo),
		prefs:          writeRepo,
		auth:           deps.Auth,
	}
}

func (m *Module) Name() string { return "users" }

func (m *Module) EnsureUser(ctx context.Context, profile contract.TelegramProfile) error {
	return m.ensure.Handle(ctx, profile)
}

func (m *Module) EnsureIdentity(ctx context.Context, id authcontract.TelegramIdentity) error {
	return m.EnsureUser(ctx, contract.TelegramProfile{
		TelegramID:   id.TelegramID,
		IsBot:        id.IsBot,
		FirstName:    id.FirstName,
		LastName:     id.LastName,
		Username:     id.Username,
		LanguageCode: id.LanguageCode,
		IsPremium:    id.IsPremium,
		PhotoURL:     id.PhotoURL,
	})
}

func (m *Module) UserID(ctx context.Context, telegramID int64) (uuid.UUID, error) {
	user, err := m.profile.Handle(ctx, telegramID)
	if err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
}

func (m *Module) UserRef(ctx context.Context, userID uuid.UUID) (contract.UserRef, error) {
	if m == nil || m.profile == nil || userID == uuid.Nil {
		return contract.UserRef{}, contract.ErrNotFound
	}
	card, err := m.profile.ByID(ctx, userID, uuid.Nil)
	if err != nil {
		return contract.UserRef{}, err
	}
	return contract.UserRef{
		ID:         card.User.ID,
		TelegramID: card.User.TelegramID,
		Username:   card.User.Username,
		FirstName:  card.User.FirstName,
	}, nil
}

func (m *Module) AllowNotifications(ctx context.Context, telegramID int64) (bool, error) {
	if m == nil || m.prefs == nil {
		return false, contract.ErrNotFound
	}
	return m.prefs.AllowNotifications(ctx, telegramID)
}

func (m *Module) AllowNotificationsByUserID(ctx context.Context, userID uuid.UUID) (bool, error) {
	if m == nil || m.prefs == nil {
		return false, contract.ErrNotFound
	}
	return m.prefs.AllowNotificationsByUserID(ctx, userID)
}

func (m *Module) SetAllowNotifications(ctx context.Context, telegramID int64, allow bool) error {
	if m == nil || m.prefs == nil {
		return contract.ErrNotFound
	}
	return m.prefs.SetAllowNotifications(ctx, telegramID, allow)
}

func (m *Module) SetAuth(auth echo.MiddlewareFunc) {
	m.auth = auth
}

func (m *Module) RegisterHTTP(g *echo.Group) {
	usershttp.Register(g, usershttp.NewHandler(m.profile, m.getSettings, m.updateSettings, m), m.auth)
}
