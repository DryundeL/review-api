package authhttp

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application"
	"review-api/internal/modules/auth/internal/domain"
)

type Handler struct {
	validate *application.ValidateInitDataHandler
}

func NewHandler(validate *application.ValidateInitDataHandler) *Handler {
	return &Handler{validate: validate}
}

func (h *Handler) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		initData, err := domain.ParseAuthorization(c.Request().Header.Get("Authorization"))
		if err != nil {
			return httperr.Unauthorized("invalid authorization")
		}
		id, err := h.validate.Handle(initData)
		if err != nil {
			return httperr.Unauthorized("invalid init data")
		}
		c.SetRequest(c.Request().WithContext(contract.WithIdentity(c.Request().Context(), id)))
		return next(c)
	}
}

func (h *Handler) Identity(c echo.Context) error {
	id, err := contract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return httperr.Unauthorized("unauthorized")
	}
	return c.JSON(http.StatusOK, identityResponse{
		TelegramID:   id.TelegramID,
		Username:     id.Username,
		LanguageCode: id.LanguageCode,
		PhotoURL:     id.PhotoURL,
		FirstName:    id.FirstName,
	})
}

type identityResponse struct {
	TelegramID   int64  `json:"telegramId"`
	Username     string `json:"username"`
	LanguageCode string `json:"languageCode"`
	PhotoURL     string `json:"photoUrl"`
	FirstName    string `json:"firstName"`
}
