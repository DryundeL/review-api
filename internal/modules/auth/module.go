package auth

import (
	"time"

	"github.com/labstack/echo/v4"

	"review-api/internal/modules/auth/internal/application"
	authhttp "review-api/internal/modules/auth/internal/delivery/http"
	"review-api/internal/modules/auth/internal/domain"
)

type Dependencies struct {
	BotToken string
	MaxAge   time.Duration
}

type Middlewares struct {
	Auth echo.MiddlewareFunc
}

type Module struct {
	Middlewares Middlewares
	handler     *authhttp.Handler
}

func NewModule(deps Dependencies) *Module {
	validator := domain.NewValidator(deps.BotToken, deps.MaxAge, nil)
	validate := application.NewValidateInitDataHandler(validator)
	h := authhttp.NewHandler(validate)
	return &Module{
		Middlewares: Middlewares{Auth: h.Middleware},
		handler:     h,
	}
}

func New(deps Dependencies) *Module {
	return NewModule(deps)
}

func (m *Module) Name() string { return "auth" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	authhttp.RegisterRoutes(g, m.handler)
}
