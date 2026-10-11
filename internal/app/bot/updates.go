package bot

import (
	"strings"

	"review-api/internal/modules/users/contract"
)

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

func startReply(languageCode string) string {
	lang := strings.ToLower(strings.TrimSpace(languageCode))
	if lang == "" || strings.HasPrefix(lang, "ru") {
		return "Профиль сохранён."
	}
	return "Profile saved."
}
