package usershttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	gamificationcontract "review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/command"
	"review-api/internal/modules/users/internal/application/query"
	"review-api/internal/modules/users/internal/domain"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type CosmeticsLookup interface {
	Cosmetics(ctx context.Context, userID uuid.UUID) (gamificationcontract.Cosmetics, error)
}

type Handler struct {
	profile   *query.GetProfileHandler
	settings  *query.GetSettingsHandler
	update    *command.UpdateSettingsHandler
	users     UserResolver
	cosmetics CosmeticsLookup
}

func NewHandler(
	profile *query.GetProfileHandler,
	settings *query.GetSettingsHandler,
	update *command.UpdateSettingsHandler,
	users UserResolver,
	cosmetics CosmeticsLookup,
) *Handler {
	return &Handler{profile: profile, settings: settings, update: update, users: users, cosmetics: cosmetics}
}

func (h *Handler) Profile(c echo.Context) error {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return httperr.Unauthorized("unauthorized")
	}
	card, err := h.profile.Own(c.Request().Context(), id.TelegramID)
	if errors.Is(err, contract.ErrNotFound) {
		return httperr.Unauthorized("unauthorized")
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, h.profileJSON(c.Request().Context(), card, true))
}

func (h *Handler) PublicProfile(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil || userID == uuid.Nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "user not found")
	}
	viewerID, err := h.optionalViewer(c)
	if err != nil {
		return err
	}
	card, err := h.profile.ByID(c.Request().Context(), userID, viewerID)
	if errors.Is(err, contract.ErrNotFound) {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "user not found")
	}
	if err != nil {
		return err
	}
	if card.IsMe {
		c.Response().Header().Set("Cache-Control", "no-store")
	}
	return c.JSON(http.StatusOK, h.profileJSON(c.Request().Context(), card, card.IsMe))
}

func (h *Handler) GetSettings(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	s, err := h.settings.Handle(c.Request().Context(), userID)
	if err != nil {
		return mapSettingsError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, settingsJSON(s))
}

func (h *Handler) PatchSettings(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	var body settingsPatchBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	s, err := h.update.Handle(c.Request().Context(), userID, command.UpdateSettingsInput{
		Language:           body.Language,
		AllowNotifications: body.AllowNotifications,
	})
	if err != nil {
		return mapSettingsError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, settingsJSON(s))
}

func (h *Handler) requireUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	if h.users == nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

func (h *Handler) optionalViewer(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return uuid.Nil, nil
	}
	if h.users == nil {
		return uuid.Nil, nil
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

type profileResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	PhotoURL       string `json:"photoUrl,omitempty"`
	FollowersCount int64  `json:"followersCount"`
	FollowingCount int64  `json:"followingCount"`
	IsFollowing    bool   `json:"isFollowing"`
	IsMe           bool   `json:"isMe"`
	TelegramID     *int64 `json:"telegramId,omitempty"`
	LanguageCode   string `json:"languageCode,omitempty"`
	IsPremium      *bool  `json:"isPremium,omitempty"`
	Background  string `json:"background,omitempty"`
	AvatarFrame string `json:"avatarFrame,omitempty"`
	Backdrop    string `json:"backdrop,omitempty"`
}

type settingsResponse struct {
	Language           string `json:"language"`
	AllowNotifications bool   `json:"allowNotifications"`
}

type settingsPatchBody struct {
	Language           *string `json:"language"`
	AllowNotifications *bool   `json:"allowNotifications"`
}

func (h *Handler) profileJSON(ctx context.Context, card query.Card, includePrivate bool) profileResponse {
	out := profileResponse{
		ID:             card.User.ID.String(),
		Username:       card.User.Username,
		FirstName:      card.User.FirstName,
		LastName:       card.User.LastName,
		PhotoURL:       card.User.PhotoURL,
		FollowersCount: card.FollowersCount,
		FollowingCount: card.FollowingCount,
		IsFollowing:    card.IsFollowing,
		IsMe:           card.IsMe,
	}
	if includePrivate {
		tid := card.User.TelegramID
		prem := card.User.IsPremium
		out.TelegramID = &tid
		out.LanguageCode = card.User.LanguageCode
		out.IsPremium = &prem
	}
	if h.cosmetics != nil {
		if cos, err := h.cosmetics.Cosmetics(ctx, card.User.ID); err == nil {
			out.Background = cos.Background
			out.AvatarFrame = cos.AvatarFrame
			out.Backdrop = cos.Backdrop
		}
	}
	return out
}

func settingsJSON(s domain.Settings) settingsResponse {
	return settingsResponse{
		Language:           s.Language,
		AllowNotifications: s.AllowNotifications,
	}
}

func mapSettingsError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid settings",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "settings not found")
	default:
		return err
	}
}
