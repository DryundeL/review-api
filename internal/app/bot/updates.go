package bot

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"review-api/internal/modules/users/contract"
)

type replyMarkup struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type inlineButton struct {
	Text   string     `json:"text"`
	WebApp webAppInfo `json:"web_app"`
}

type webAppInfo struct {
	URL string `json:"url"`
}

func startProfile(update Update) (contract.TelegramProfile, int64, bool) {
	if update.Message == nil || update.Message.From == nil || !isStartCommand(update.Message.Text) {
		return contract.TelegramProfile{}, 0, false
	}
	from := update.Message.From
	var chatID int64
	if update.Message.Chat.Type == "private" && update.Message.Chat.ID > 0 {
		chatID = update.Message.Chat.ID
	}
	return contract.TelegramProfile{
		TelegramID:   from.ID,
		IsBot:        from.IsBot,
		FirstName:    from.FirstName,
		LastName:     from.LastName,
		Username:     from.Username,
		LanguageCode: from.LanguageCode,
		IsPremium:    from.IsPremium,
	}, chatID, true
}

func isStartCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	command, _, _ := strings.Cut(fields[0], "@")
	return strings.EqualFold(command, "/start")
}

func parseMiniAppURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("MINI_APP_URL must be an https URL without credentials")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		return "", fmt.Errorf("MINI_APP_URL must be a public https URL")
	}
	return parsed.String(), nil
}

func startMessage(profile contract.TelegramProfile, appURL string) (string, replyMarkup) {
	ru := isRussian(profile.LanguageCode)
	name := strings.TrimSpace(visibleText(profile.FirstName, 64) + " " + visibleText(profile.LastName, 64))
	if name == "" {
		name = "there"
		if ru {
			name = "друг"
		}
	}

	var text strings.Builder
	if ru {
		fmt.Fprintf(&text, "Привет, %s", name)
	} else {
		fmt.Fprintf(&text, "Hi, %s", name)
	}
	if username := visibleUsername(profile.Username); username != "" {
		fmt.Fprintf(&text, " (@%s)", username)
	}
	text.WriteString(".\n\n")
	fmt.Fprintf(&text, "Telegram ID: %d\n", profile.TelegramID)
	if language := visibleText(profile.LanguageCode, 16); language != "" {
		if ru {
			fmt.Fprintf(&text, "Язык: %s\n", language)
		} else {
			fmt.Fprintf(&text, "Language: %s\n", language)
		}
	}
	if profile.IsPremium {
		if ru {
			text.WriteString("Premium: да\n")
		} else {
			text.WriteString("Premium: yes\n")
		}
	}
	if ru {
		text.WriteString("\nТвои возможности ниже.")
	} else {
		text.WriteString("\nYour options are below.")
	}

	label := "Open mini app"
	if ru {
		label = "Открыть мини-приложение"
	}
	markup := replyMarkup{InlineKeyboard: [][]inlineButton{{{
		Text:   label,
		WebApp: webAppInfo{URL: appURL},
	}}}}
	return text.String(), markup
}

func isRussian(languageCode string) bool {
	lang := strings.ToLower(strings.TrimSpace(languageCode))
	return lang == "" || strings.HasPrefix(lang, "ru")
}

func visibleUsername(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "@")
	if value == "" || len(value) > 32 {
		return ""
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return ""
		}
	}
	return value
}

func visibleText(value string, max int) string {
	value = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\u200d' || r == '\ufe0f' {
			return r
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, value))
	if utf8.RuneCountInString(value) > max {
		value = strings.TrimSpace(string([]rune(value)[:max]))
	}
	return value
}
