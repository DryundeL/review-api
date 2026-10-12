package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

var errJWT = errors.New("invalid jwt")

// SignJWT issues an HS256 token whose subject is the Telegram user id.
func SignJWT(secret []byte, telegramID int64, expires time.Time) (string, error) {
	if len(secret) < 32 || telegramID <= 0 || expires.IsZero() {
		return "", errJWT
	}
	payload, err := json.Marshal(struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}{
		Sub: strconv.FormatInt(telegramID, 10),
		Exp: expires.Unix(),
	})
	if err != nil {
		return "", errJWT
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// ParseJWT checks the signature and expiry. It returns the Telegram user id.
func ParseJWT(secret []byte, token string, now time.Time) (int64, error) {
	if len(secret) < 32 || len(token) < 40 || len(token) > 2048 {
		return 0, errJWT
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[0]) > 128 || len(parts[1]) > 512 || len(parts[2]) > 128 {
		return 0, errJWT
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return 0, errJWT
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return 0, errJWT
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errJWT
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(headerJSON, &header) != nil || header.Alg != "HS256" {
		return 0, errJWT
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, errJWT
	}
	var claims struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if json.Unmarshal(payloadJSON, &claims) != nil {
		return 0, errJWT
	}
	id, err := strconv.ParseInt(claims.Sub, 10, 64)
	if err != nil || id <= 0 || !now.Before(time.Unix(claims.Exp, 0)) {
		return 0, errJWT
	}
	return id, nil
}
