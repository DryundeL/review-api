package httpserver

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	code := "INTERNAL"
	message := "internal server error"
	var details any

	var api *httperr.Error
	if errors.As(err, &api) && api != nil {
		status = api.Status
		code = api.Code
		message = api.Message
		details = api.Details
		if writeErr := c.JSON(status, httperr.Body{Error: httperr.Detail{
			Code:    code,
			Message: message,
			Details: details,
		}}); writeErr != nil {
			c.Logger().Error(writeErr)
		}
		return
	}

	var he *echo.HTTPError
	if errors.As(err, &he) && he != nil {
		status = he.Code
		switch status {
		case http.StatusNotFound:
			code, message = "NOT_FOUND", http.StatusText(status)
		case http.StatusMethodNotAllowed:
			code, message = "METHOD_NOT_ALLOWED", http.StatusText(status)
		default:
			message = http.StatusText(status)
			if msg, ok := he.Message.(string); ok && msg != "" && status < http.StatusInternalServerError {
				message = msg
			}
		}
	}

	if writeErr := c.JSON(status, httperr.Body{Error: httperr.Detail{
		Code:    code,
		Message: message,
	}}); writeErr != nil {
		c.Logger().Error(writeErr)
	}
}
