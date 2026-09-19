package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type ctxKey int

const loggerKey ctxKey = 1

func LoggerMiddleware(base *slog.Logger) echo.MiddlewareFunc {
	if base == nil {
		base = slog.Default()
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Response().Header().Get(echo.HeaderXRequestID)
			if reqID == "" {
				reqID = c.Request().Header.Get(echo.HeaderXRequestID)
			}
			logger := base.With("request_id", reqID)
			c.SetRequest(c.Request().WithContext(context.WithValue(c.Request().Context(), loggerKey, logger)))

			start := time.Now()
			err := next(c)
			logger.Info("http request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", c.Response().Status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return err
		}
	}
}

func GetLoggerFromContext(r *http.Request) *slog.Logger {
	if r == nil {
		return slog.Default()
	}
	if logger, ok := r.Context().Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
