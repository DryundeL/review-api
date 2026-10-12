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

type pollLoginResponse struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
}
