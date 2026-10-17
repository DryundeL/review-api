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
	Keyboard       [][]keyboardButton `json:"keyboard"`
	ResizeKeyboard bool               `json:"resize_keyboard"`
	IsPersistent   bool               `json:"is_persistent"`
}

type keyboardButton struct {
	Text string `json:"text"`
}

type webAppInfo struct {
	URL string `json:"url"`
}

type botAction int

const (
	actionNone botAction = iota
	actionStart
	actionMute
	actionUnmute
)

func messageProfile(update Update) (contract.TelegramProfile, int64, bool) {
	if update.Message == nil || update.Message.From == nil {
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

func startProfile(update Update) (contract.TelegramProfile, int64, bool) {
	profile, chatID, ok := messageProfile(update)
	if !ok || classifyMessage(update.Message.Text, profile.LanguageCode) != actionStart {
		return contract.TelegramProfile{}, 0, false
	}
	return profile, chatID, true
}

func classifyMessage(text, languageCode string) botAction {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return actionNone
	}
	command, _, _ := strings.Cut(fields[0], "@")
	switch strings.ToLower(command) {
	case "/start":
		return actionStart
	case "/mute":
		return actionMute
	case "/unmute":
		return actionUnmute
	}

	ru := isRussian(languageCode)
	switch strings.TrimSpace(text) {
	case btnStart(ru):
		return actionStart
	case btnMute(ru):
		return actionMute
	case btnUnmute(ru):
		return actionUnmute
	}
	return actionNone
}

func isStartCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
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

// parseMiniAppName validates the blue chat menu button label (Telegram: 1–16 chars).
func parseMiniAppName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 16 {
		return "", fmt.Errorf("MINI_APP_NAME must be 1–16 characters")
	}
	return name, nil
}

type startView struct {
	Text       string
	Markup     replyMarkup
	MenuButton string
}

func startMessage(profile contract.TelegramProfile, appName string, muted bool) startView {
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
	text.WriteString("\n")
	text.WriteString(notificationsStatus(ru, muted))
	if ru {
		text.WriteString("\n\nКоманды — кнопками ниже или через /.")
	} else {
		text.WriteString("\n\nUse the buttons below or / commands.")
	}

	return startView{
		Text:       text.String(),
		Markup:     commandsKeyboard(ru, muted),
		MenuButton: appName,
	}
}

func commandsKeyboard(ru, muted bool) replyMarkup {
	toggle := btnMute(ru)
	if muted {
		toggle = btnUnmute(ru)
	}
	return replyMarkup{
		Keyboard: [][]keyboardButton{
			{{Text: btnStart(ru)}, {Text: toggle}},
		},
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

func btnStart(ru bool) string {
	if ru {
		return "Старт"
	}
	return "Start"
}

func btnMute(ru bool) string {
	if ru {
		return "Выключить уведомления"
	}
	return "Mute notifications"
}

func btnUnmute(ru bool) string {
	if ru {
		return "Включить уведомления"
	}
	return "Unmute notifications"
}

func notificationsStatus(ru bool, muted bool) string {
	if muted {
		if ru {
			return "Уведомления: выключены."
		}
		return "Notifications: muted."
	}
	if ru {
		return "Уведомления: включены."
	}
	return "Notifications: on."
}

func muteReply(ru, muted bool) string {
	if muted {
		if ru {
			return "Уведомления выключены. Бот больше не будет присылать пуши."
		}
		return "Notifications muted. The bot will not send pushes."
	}
	if ru {
		return "Уведомления включены."
	}
	return "Notifications enabled."
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
