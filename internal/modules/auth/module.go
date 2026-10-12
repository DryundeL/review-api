package auth

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application"
	authhttp "review-api/internal/modules/auth/internal/delivery/http"
	"review-api/internal/modules/auth/internal/domain"
	"review-api/internal/modules/auth/internal/infrastructure/login"
)

type Dependencies struct {
	BotToken    string
	MaxAge      time.Duration
	Redis       *redis.Client
	JWTSecret   string
	BotUsername string
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
	h := authhttp.NewHandler(validate, svc, []byte(deps.JWTSecret))
	return &Module{
		Middlewares: Middlewares{Auth: h.Middleware, Bearer: h.Bearer},
		handler:     h,
		login:       svc,
	}
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
