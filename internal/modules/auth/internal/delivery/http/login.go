package authhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application"
	"review-api/internal/modules/auth/internal/domain"
)

func (h *Handler) Bearer(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token, ok := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
		if !ok {
			return httperr.Unauthorized("unauthorized")
		}
		telegramID, err := domain.ParseJWT(h.secret, token, time.Now())
		if err != nil {
			return httperr.Unauthorized("unauthorized")
		}
		ctx := contract.WithIdentity(c.Request().Context(), contract.TelegramIdentity{TelegramID: telegramID})
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}

func (h *Handler) Telegram(c echo.Context) error {
	if h.validate == nil || len(h.secret) < 32 {
		return errors.New("login unavailable")
	}
	initData, err := domain.ParseAuthorization(c.Request().Header.Get(echo.HeaderAuthorization))
	if err != nil {
		return httperr.Unauthorized("invalid authorization")
	}
	id, err := h.validate.Handle(initData)
	if err != nil {
		return httperr.Unauthorized("invalid init data")
	}
	if h.ensure != nil {
		if err := h.ensure(c.Request().Context(), id); err != nil {
			if errors.Is(err, contract.ErrUnauthorized) {
				return httperr.Unauthorized("unauthorized")
			}
			return err
		}
	}
	token, err := domain.SignJWT(h.secret, id.TelegramID, time.Now().Add(h.maxAge))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, telegramResponse{Token: token})
}

func (h *Handler) StartLogin(c echo.Context) error {
	if h.login == nil {
		return errors.New("login unavailable")
	}
	started, err := h.login.Start(c.Request().Context(), c.Request().UserAgent())
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, startLoginResponse{
		LoginID:   started.ID,
		URL:       started.URL,
		ExpiresIn: started.ExpiresIn,
	})
}

func (h *Handler) PollLogin(c echo.Context) error {
	if h.login == nil {
		return errors.New("login unavailable")
	}
	var req pollLoginRequest
	dec := json.NewDecoder(http.MaxBytesReader(c.Response().Writer, c.Request().Body, 1024))
	if err := dec.Decode(&req); err != nil || !contract.ValidLoginID(req.LoginID) {
		return httperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid login id")
	}
	status, token, err := h.login.Poll(c.Request().Context(), req.LoginID)
	if errors.Is(err, contract.ErrLoginNotFound) {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "login not found")
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, pollLoginResponse{Status: status, Token: token})
}

func (h *Handler) StartEmailLogin(c echo.Context) error {
	if h.email == nil {
		return errors.New("email login unavailable")
	}
	var req emailLoginRequest
	dec := json.NewDecoder(http.MaxBytesReader(c.Response().Writer, c.Request().Body, 1024))
	if err := dec.Decode(&req); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	err := h.email.Start(c.Request().Context(), req.Email)
	switch {
	case errors.Is(err, application.ErrEmailCooldown):
		return httperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "verification code recently sent")
	case errors.Is(err, contract.ErrInvalidEmail):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid email")
	case errors.Is(err, application.ErrEmailUnavailable):
		return httperr.New(http.StatusServiceUnavailable, "EMAIL_UNAVAILABLE", "email delivery unavailable")
	case err != nil:
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusAccepted, map[string]int{"expiresIn": 600})
}

func (h *Handler) ConfirmEmailLogin(c echo.Context) error {
	if h.email == nil {
		return errors.New("email login unavailable")
	}
	var req emailConfirmRequest
	dec := json.NewDecoder(http.MaxBytesReader(c.Response().Writer, c.Request().Body, 1024))
	if err := dec.Decode(&req); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	token, err := h.email.Confirm(c.Request().Context(), req.Email, req.Code)
	if errors.Is(err, application.ErrEmailCodeInvalid) {
		return httperr.Unauthorized("invalid verification code")
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, telegramResponse{Token: token})
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, " ") {
		return "", false
	}
	return token, true
}

type telegramResponse struct {
	Token string `json:"token"`
}

type startLoginResponse struct {
	LoginID   string `json:"loginId"`
	URL       string `json:"url"`
	ExpiresIn int    `json:"expiresIn"`
}

type pollLoginRequest struct {
	LoginID string `json:"loginId"`
}

type emailLoginRequest struct {
	Email string `json:"email"`
}

type emailConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type pollLoginResponse struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
}
