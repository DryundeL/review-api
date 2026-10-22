package auth

import (
	"context"
	"errors"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application"
	authhttp "review-api/internal/modules/auth/internal/delivery/http"
	"review-api/internal/modules/auth/internal/domain"
	"review-api/internal/modules/auth/internal/infrastructure/login"
	"review-api/internal/platform/authcode"
)

type Dependencies struct {
	BotToken      string
	MaxAge        time.Duration
	Redis         *redis.Client
	JWTSecret     string
	BotUsername   string
	EnsureUser    func(context.Context, contract.TelegramIdentity) error
	EmailResolver EmailResolver
	EmailSender   application.EmailSender
}

type EmailResolver interface {
	ResolveLoginEmail(ctx context.Context, raw string) (normalized string, telegramID int64, found bool, err error)
}

type Middlewares struct {
	Auth   echo.MiddlewareFunc
	Bearer echo.MiddlewareFunc
}

type Module struct {
	Middlewares Middlewares
	handler     *authhttp.Handler
	login       *application.LoginService
}

func NewModule(deps Dependencies) *Module {
	validator := domain.NewValidator(deps.BotToken, deps.MaxAge, nil)
	validate := application.NewValidateInitDataHandler(validator)
	var svc *application.LoginService
	if deps.Redis != nil {
		svc = application.NewLoginService(login.NewStore(deps.Redis), []byte(deps.JWTSecret), deps.BotUsername, deps.MaxAge)
	}
	h := authhttp.NewHandler(validate, svc, []byte(deps.JWTSecret), deps.MaxAge, deps.EnsureUser)
	emailCodes := &emailChallengeAdapter{store: authcode.New(deps.Redis, []byte(deps.JWTSecret), "auth:email")}
	h.SetEmailLogin(application.NewEmailLoginService(deps.EmailResolver, emailCodes, deps.EmailSender, []byte(deps.JWTSecret), deps.MaxAge))
	return &Module{
		Middlewares: Middlewares{Auth: h.Middleware, Bearer: h.Bearer},
		handler:     h,
		login:       svc,
	}
}

type emailChallengeAdapter struct {
	store *authcode.Store
}

func (a *emailChallengeAdapter) Issue(ctx context.Context, subject, value string) (string, error) {
	code, err := a.store.Issue(ctx, subject, value)
	if errors.Is(err, authcode.ErrCooldown) {
		return "", application.ErrEmailCooldown
	}
	return code, err
}

func (a *emailChallengeAdapter) Verify(ctx context.Context, subject, code string) (string, error) {
	return a.store.Verify(ctx, subject, code)
}

func (a *emailChallengeAdapter) Cancel(ctx context.Context, subject string) {
	a.store.Cancel(ctx, subject)
}

func New(deps Dependencies) *Module {
	return NewModule(deps)
}

func (m *Module) Name() string { return "auth" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	authhttp.RegisterRoutes(g, m.handler)
}

func (m *Module) GetLogin(ctx context.Context, id string) (contract.Login, error) {
	if m == nil || m.login == nil {
		return contract.Login{}, contract.ErrLoginNotFound
	}
	return m.login.Get(ctx, id)
}

func (m *Module) ConfirmLogin(ctx context.Context, id string, telegramID int64) error {
	if m == nil || m.login == nil {
		return contract.ErrLoginNotFound
	}
	return m.login.Confirm(ctx, id, telegramID)
}

func (m *Module) DenyLogin(ctx context.Context, id string) error {
	if m == nil || m.login == nil {
		return contract.ErrLoginNotFound
	}
	return m.login.Deny(ctx, id)
}
