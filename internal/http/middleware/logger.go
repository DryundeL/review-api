package middleware

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"review-api/internal/platform/httpserver"
)

func LoggerMiddleware(logger *slog.Logger) echo.MiddlewareFunc {
	return httpserver.LoggerMiddleware(logger)
}

func GetLoggerFromContext(r *http.Request) *slog.Logger {
	return httpserver.GetLoggerFromContext(r)
}
