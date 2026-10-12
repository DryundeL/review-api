package authhttp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application"
	"review-api/internal/modules/auth/internal/domain"
)

func TestTelegramIssuesJWTWithoutBot(t *testing.T) {
	t.Parallel()
	secret := []byte(strings.Repeat("s", 32))
	now := time.Now().UTC()
	const botToken = "123456:abcDEF_token-1"
	validate := application.NewValidateInitDataHandler(domain.NewValidator(botToken, time.Hour, func() time.Time { return now }))
	var saved contract.TelegramIdentity
	h := NewHandler(validate, nil, secret, time.Hour, func(_ context.Context, id contract.TelegramIdentity) error {
		saved = id
		return nil
	})

	initData := signInitData(t, botToken, now.Unix(), map[string]any{
		"id": int64(42), "first_name": "Ann", "last_name": "Lee", "username": "ann", "is_premium": true,
	})
	e := echo.New()
	e.POST("/v1/auth/telegram", h.Telegram)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/telegram", nil)
	req.Header.Set("Authorization", "tma "+initData)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body telegramResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	id, err := domain.ParseJWT(secret, body.Token, now)
	if err != nil || id != 42 {
		t.Fatalf("token id=%d err=%v", id, err)
	}
	if saved.FirstName != "Ann" || saved.LastName != "Lee" || !saved.IsPremium || saved.Username != "ann" {
		t.Fatalf("saved %+v", saved)
	}
}

func signInitData(t *testing.T, token string, authUnix int64, user map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}
	fields := url.Values{}
	fields.Set("auth_date", strconv.FormatInt(authUnix, 10))
	fields.Set("user", string(raw))
	fields.Set("query_id", "AAE")
	check := "auth_date=" + fields.Get("auth_date") + "\nquery_id=" + fields.Get("query_id") + "\nuser=" + fields.Get("user")
	mac := hmac.New(sha256.New, hmacSHA256([]byte("WebAppData"), []byte(token)))
	_, _ = mac.Write([]byte(check))
	fields.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return fields.Encode()
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}
