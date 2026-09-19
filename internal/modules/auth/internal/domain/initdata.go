package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"review-api/internal/modules/auth/contract"
)

const webAppDataKey = "WebAppData"

type Clock func() time.Time

type Validator struct {
	botToken string
	maxAge   time.Duration
	now      Clock
}

func NewValidator(botToken string, maxAge time.Duration, now Clock) *Validator {
	if now == nil {
		now = time.Now
	}
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &Validator{botToken: botToken, maxAge: maxAge, now: now}
}

func (v *Validator) Validate(initData string) (contract.TelegramIdentity, error) {
	if strings.TrimSpace(initData) == "" {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}
	values, err := url.ParseQuery(initData)
	if err != nil {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}
	hash := values.Get("hash")
	if hash == "" {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}

	pairs := make([]string, 0, len(values))
	for k, vs := range values {
		if k == "hash" || k == "signature" {
			continue
		}
		if len(vs) == 0 {
			continue
		}
		pairs = append(pairs, k+"="+vs[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secret := hmacSHA256([]byte(webAppDataKey), []byte(v.botToken))
	sum := hmacSHA256(secret, []byte(dataCheckString))
	expected := hex.EncodeToString(sum)

	got, err := hex.DecodeString(hash)
	if err != nil {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}
	want, err := hex.DecodeString(expected)
	if err != nil || !hmac.Equal(got, want) {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}

	authDateRaw := values.Get("auth_date")
	authUnix, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}
	authAt := time.Unix(authUnix, 0).UTC()
	now := v.now().UTC()
	if authAt.After(now.Add(60 * time.Second)) {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}
	if now.Sub(authAt) > v.maxAge {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}

	userJSON := values.Get("user")
	if userJSON == "" {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}
	var user struct {
		ID           int64  `json:"id"`
		Username     string `json:"username"`
		LanguageCode string `json:"language_code"`
		PhotoURL     string `json:"photo_url"`
		FirstName    string `json:"first_name"`
	}
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil || user.ID == 0 {
		return contract.TelegramIdentity{}, contract.ErrUnauthorized
	}

	return contract.TelegramIdentity{
		TelegramID:   user.ID,
		Username:     user.Username,
		LanguageCode: user.LanguageCode,
		PhotoURL:     user.PhotoURL,
		FirstName:    user.FirstName,
	}, nil
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func ParseAuthorization(header string) (string, error) {
	const prefix = "tma "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("%w", contract.ErrUnauthorized)
	}
	initData := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if initData == "" {
		return "", contract.ErrUnauthorized
	}
	return initData, nil
}
