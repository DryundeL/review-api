package usershttp

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/query"
)

type Handler struct {
	profile *query.GetProfileHandler
}

func NewHandler(profile *query.GetProfileHandler) *Handler {
	return &Handler{profile: profile}
}

func (h *Handler) Profile(c echo.Context) error {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return httperr.Unauthorized("unauthorized")
	}
	user, err := h.profile.Handle(c.Request().Context(), id.TelegramID)
	if errors.Is(err, contract.ErrNotFound) {
		return httperr.Unauthorized("unauthorized")
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, profileResponse{
		ID:           user.ID.String(),
		TelegramID:   user.TelegramID,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		LanguageCode: user.LanguageCode,
		IsPremium:    user.IsPremium,
	})
}

type profileResponse struct {
	ID           string `json:"id"`
	TelegramID   int64  `json:"telegramId"`
	Username     string `json:"username"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	LanguageCode string `json:"languageCode"`
	IsPremium    bool   `json:"isPremium"`
}
