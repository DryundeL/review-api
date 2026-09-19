package httpserver

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type Options struct {
	Logger      *slog.Logger
	DB          *pgxpool.Pool
	Redis       *redis.Client
	Service     string
	Version     string
	CORSOrigins []string
	Env         string
}

func NewMux(opts Options) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ErrorHandler

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(corsMiddleware(opts.CORSOrigins))
	e.Use(LoggerMiddleware(opts.Logger))

	health := NewHealthHandler(opts.DB, opts.Redis, opts.Service, opts.Version)
	RegisterHealthRoutes(e, health)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	return e
}

func corsMiddleware(origins []string) echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			if len(origins) == 0 {
				return true, nil
			}
			for _, allowed := range origins {
				if strings.EqualFold(allowed, origin) {
					return true, nil
				}
			}
			return false, nil
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowCredentials: true,
	})
}
