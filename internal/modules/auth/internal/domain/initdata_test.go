package domain_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/domain"
)

const testToken = "test-bot-token"

func TestValidator_ValidInitData(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC)
	v := domain.NewValidator(testToken, 24*time.Hour, func() time.Time { return now })

	initData := mustSign(t, testToken, now.Unix(), map[string]any{
		"id":            int64(42),
		"username":      "alice",
		"language_code": "ru",
		"first_name":    "Alice",
	})

	id, err := v.Validate(initData)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if id.TelegramID != 42 || id.Username != "alice" || id.LanguageCode != "ru" {
		t.Fatalf("identity = %+v", id)
	}
}

func TestValidator_BadHash(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	v := domain.NewValidator(testToken, 24*time.Hour, func() time.Time { return now })
	initData := mustSign(t, testToken, now.Unix(), map[string]any{"id": int64(1), "username": "x"})
	initData += "tampered"

	_, err := v.Validate(initData)
	if err != contract.ErrUnauthorized {
		t.Fatalf("want unauthorized, got %v", err)
	}
}

func TestValidator_Expired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC)
	v := domain.NewValidator(testToken, time.Hour, func() time.Time { return now })
	initData := mustSign(t, testToken, now.Add(-2*time.Hour).Unix(), map[string]any{"id": int64(1)})

	_, err := v.Validate(initData)
	if err != contract.ErrUnauthorized {
		t.Fatalf("want unauthorized, got %v", err)
	}
}

func TestParseAuthorization(t *testing.T) {
	t.Parallel()
	got, err := domain.ParseAuthorization("tma auth_date=1")
	if err != nil || got != "auth_date=1" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := domain.ParseAuthorization("Bearer x"); err == nil {
		t.Fatal("expected error")
	}
}

func mustSign(t *testing.T, token string, authUnix int64, user map[string]any) string {
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

	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	sum := hmacSHA256(secret, []byte(check))
	fields.Set("hash", hex.EncodeToString(sum))
	return fields.Encode()
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}
