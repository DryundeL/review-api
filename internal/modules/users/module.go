package users

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	authcontract "review-api/internal/modules/auth/contract"
	gamificationcontract "review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/command"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/application/query"
	usershttp "review-api/internal/modules/users/internal/delivery/http"
	"review-api/internal/modules/users/internal/domain"
	"review-api/internal/modules/users/internal/infrastructure/read"
	"review-api/internal/modules/users/internal/infrastructure/write"
	"review-api/internal/platform/authcode"
)

type CosmeticsLookup interface {
	Cosmetics(ctx context.Context, userID uuid.UUID) (gamificationcontract.Cosmetics, error)
}

type Dependencies struct {
	Pool        *pgxpool.Pool
	Auth        echo.MiddlewareFunc
	Redis       *redis.Client
	AuthSecret  string
	EmailSender command.EmailSender
}

type Module struct {
	ensure         *command.EnsureUserHandler
	profile        *query.GetProfileHandler
	getSettings    *query.GetSettingsHandler
	updateSettings *command.UpdateSettingsHandler
	email          *command.EmailHandler
	emailRepo      ports.EmailRepository
	prefs          ports.UserRepository
	cosmetics      CosmeticsLookup
	auth           echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	writeRepo := write.NewRepository(deps.Pool)
	codes := &challengeAdapter{store: authcode.New(deps.Redis, []byte(deps.AuthSecret), "users:email")}
	return &Module{
		ensure:         command.NewEnsureUserHandler(writeRepo, nil),
		profile:        query.NewGetProfileHandler(read.NewRepository(deps.Pool)),
		getSettings:    query.NewGetSettingsHandler(writeRepo),
		updateSettings: command.NewUpdateSettingsHandler(writeRepo),
		email:          command.NewEmailHandler(writeRepo, codes, deps.EmailSender),
		emailRepo:      writeRepo,
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

func (m *Module) FindTelegramIDByEmail(ctx context.Context, rawEmail string) (int64, error) {
	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return 0, err
	}
	return m.emailRepo.FindTelegramIDByEmail(ctx, email)
}

func (m *Module) ResolveLoginEmail(ctx context.Context, rawEmail string) (string, int64, bool, error) {
	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return "", 0, false, err
	}
	telegramID, err := m.emailRepo.FindTelegramIDByEmail(ctx, email)
	if errors.Is(err, contract.ErrNotFound) {
		return email, 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	return email, telegramID, true, nil
}

func (m *Module) SetCosmetics(c CosmeticsLookup) {
	m.cosmetics = c
}

func (m *Module) RegisterHTTP(g *echo.Group) {
	handler := usershttp.NewHandler(m.profile, m.getSettings, m.updateSettings, m, m.cosmetics)
	handler.SetEmailHandler(m.email)
	usershttp.Register(g, handler, m.auth)
}

type challengeAdapter struct {
	store *authcode.Store
}

func (a *challengeAdapter) Issue(ctx context.Context, subject, email string) (string, error) {
	code, err := a.store.Issue(ctx, subject, email)
	if errors.Is(err, authcode.ErrCooldown) {
		return "", command.ErrCooldown
	}
	return code, err
}

func (a *challengeAdapter) Verify(ctx context.Context, subject, code string) (string, error) {
	return a.store.Verify(ctx, subject, code)
}

func (a *challengeAdapter) Cancel(ctx context.Context, subject string) {
	a.store.Cancel(ctx, subject)
}
