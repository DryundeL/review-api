package domain

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
)

const (
	maxNameLen     = 64
	maxUsernameLen = 32
	maxLangLen     = 16
	maxPhotoURLLen = 2048
)

// User is the stored profile. Optional strings are empty when the column is NULL.
type User struct {
	ID           uuid.UUID
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	IsPremium    bool
	PhotoURL     string
	Email        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewUser(p contract.TelegramProfile, now time.Time) (User, error) {
	if p.IsBot || p.TelegramID <= 0 {
		return User{}, contract.ErrInvalidProfile
	}
	first, err := cleanName(p.FirstName, true)
	if err != nil {
		return User{}, err
	}
	last, err := cleanName(p.LastName, false)
	if err != nil {
		return User{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	return User{
		ID:           uuid.New(),
		TelegramID:   p.TelegramID,
		Username:     cleanUsername(p.Username),
		FirstName:    first,
		LastName:     last,
		LanguageCode: cleanLanguage(p.LanguageCode),
		IsPremium:    p.IsPremium,
		PhotoURL:     cleanPhotoURL(p.PhotoURL),
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func cleanName(value string, required bool) (string, error) {
	value = strings.TrimSpace(strings.Map(dropUnsafeRune, value))
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", contract.ErrInvalidProfile
		}
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxNameLen {
		value = strings.TrimSpace(string([]rune(value)[:maxNameLen]))
	}
	if value == "" {
		if required {
			return "", contract.ErrInvalidProfile
		}
		return "", nil
	}
	return value, nil
}

func dropUnsafeRune(r rune) rune {
	if r == '\u200d' || r == '\ufe0f' {
		return r
	}
	if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
		return -1
	}
	return r
}

func cleanUsername(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "@")
	if value == "" || len(value) > maxUsernameLen {
		return ""
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return ""
		}
	}
	return value
}

func cleanLanguage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || len(value) > maxLangLen || strings.HasSuffix(value, "-") {
		return ""
	}
	for i, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-' && i > 0:
		default:
			return ""
		}
	}
	return value
}

// DefaultLanguage returns a settings language from Telegram language_code.
func DefaultLanguage(telegramLang string) string {
	if cleaned := cleanLanguage(telegramLang); cleaned != "" {
		return cleaned
	}
	return "en"
}

// SettingsLanguage validates an explicit app-locale override.
func SettingsLanguage(value string) (string, error) {
	cleaned := cleanLanguage(value)
	if cleaned == "" {
		return "", contract.FieldError{Field: "language"}
	}
	return cleaned, nil
}

// Settings is the per-user app preference row (not the Telegram identity snapshot).
type Settings struct {
	UserID             uuid.UUID
	Language           string
	AllowNotifications bool
}

// cleanPhotoURL keeps only https Telegram profile links from signed initData.
func cleanPhotoURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxPhotoURLLen {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return ""
	}
	return parsed.String()
}
