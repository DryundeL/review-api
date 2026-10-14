package httpserver

import (
	"crypto/sha256"
	"crypto/subtle"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

// AdminTokenHeader is the only credential for /v1/admin routes.
const AdminTokenHeader = "X-Admin-Token"

// AdminToken rejects the request unless the header matches the configured secret.
// The comparison does not depend on the length of the presented token.
func AdminToken(token string) echo.MiddlewareFunc {
	want := sha256.Sum256([]byte(token))
	configured := len(token) >= 32
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !configured {
				return httperr.Unauthorized("unauthorized")
			}
			got := sha256.Sum256([]byte(c.Request().Header.Get(AdminTokenHeader)))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				return httperr.Unauthorized("unauthorized")
			}
			return next(c)
		}
	}
}
