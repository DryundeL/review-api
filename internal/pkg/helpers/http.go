package helpers

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/labstack/echo/v4"

	httperrors "review-api/internal/http/errors"
	"review-api/internal/http/httperr"
	httpmiddleware "review-api/internal/http/middleware"
	"review-api/internal/pkg/validator"
)

func ParseInt64Param(c echo.Context, paramName string) (int64, error) {
	str := c.Param(paramName)
	if str == "" {
		return 0, fmt.Errorf("missing %s", paramName)
	}
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", paramName)
	}
	return val, nil
}

func DecodeAndValidateJSON(c echo.Context, req any) error {
	if err := json.NewDecoder(c.Request().Body).Decode(req); err != nil {
		return httperrors.ErrInvalidRequestBody
	}
	if err := validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

func WriteError(c echo.Context, err error) error {
	logger := httpmiddleware.GetLoggerFromContext(c.Request())
	mapped := httperrors.Map(err, logger)
	if mapped.Status >= 500 {
		logger.Error("request error", "error", err, "status", mapped.Status)
	}
	return httperr.New(mapped.Status, mapped.Code, mapped.Message)
}

func HandleServiceError(c echo.Context, err error, msg string, keyvals ...any) error {
	if err == nil {
		return nil
	}
	logger := httpmiddleware.GetLoggerFromContext(c.Request())
	args := make([]any, 0, len(keyvals)+2)
	args = append(args, "error", err)
	args = append(args, keyvals...)
	logger.Error(msg, args...)
	mapped := httperrors.Map(err, logger)
	return httperr.New(mapped.Status, mapped.Code, mapped.Message)
}
