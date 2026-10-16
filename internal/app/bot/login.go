package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users/contract"
)

type loginGateway interface {
	GetLogin(ctx context.Context, id string) (authcontract.Login, error)
	ConfirmLogin(ctx context.Context, id string, telegramID int64) error
	DenyLogin(ctx context.Context, id string) error
}

type inlineKeyboard struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type inlineButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

func loginStartID(update Update) (string, bool) {
	if update.Message == nil || update.Message.From == nil || update.Message.Chat.Type != "private" {
		return "", false
	}
	fields := strings.Fields(update.Message.Text)
	if len(fields) < 2 || !isStartCommand(fields[0]) {
		return "", false
	}
	id, ok := strings.CutPrefix(fields[1], "login_")
	if !ok || !authcontract.ValidLoginID(id) {
		return "", false
	}
	return id, true
}

func (a *App) handleLoginStart(ctx context.Context, update Update, id string) error {
	if a.logins == nil {
		return errors.New("login store unavailable")
	}
	from := update.Message.From
	chatID := update.Message.Chat.ID
	if err := a.openPrivateChat(ctx, telegramProfile(from), chatID); err != nil {
		return err
	}
	login, err := a.logins.GetLogin(ctx, id)
	if errors.Is(err, authcontract.ErrLoginNotFound) {
		return a.send(ctx, chatID, loginNotice(from.LanguageCode, "expired"), nil)
	}
	if err != nil {
		return err
	}
	switch login.Status {
	case authcontract.LoginPending:
		text, markup := loginPrompt(login.Browser, login.Device, from.LanguageCode, id)
		return a.send(ctx, chatID, text, markup)
	case authcontract.LoginConfirmed:
		return a.send(ctx, chatID, loginNotice(from.LanguageCode, "confirmed"), nil)
	case authcontract.LoginDenied:
		return a.send(ctx, chatID, loginNotice(from.LanguageCode, "denied"), nil)
	default:
		return a.send(ctx, chatID, loginNotice(from.LanguageCode, "expired"), nil)
	}
}

func (a *App) handleLoginCallback(ctx context.Context, cb *CallbackQuery) error {
	if cb == nil || cb.From == nil || cb.From.IsBot {
		return nil
	}
	approve, id, ok := parseLoginCallback(cb.Data)
	if !ok {
		return a.answer(ctx, cb.ID, "")
	}
	if a.logins == nil {
		return errors.New("login store unavailable")
	}
	if cb.Message == nil || cb.Message.Chat.Type != "private" || cb.Message.Chat.ID != cb.From.ID {
		return a.answer(ctx, cb.ID, "")
	}
	lang := cb.From.LanguageCode
	var done string
	var err error
	if approve {
		if err = a.users.EnsureUser(ctx, telegramProfile(cb.From)); err != nil {
			if errors.Is(err, contract.ErrInvalidProfile) {
				return a.answer(ctx, cb.ID, "")
			}
			return err
		}
		err = a.logins.ConfirmLogin(ctx, id, cb.From.ID)
		done = loginNotice(lang, "confirmed")
	} else {
		err = a.logins.DenyLogin(ctx, id)
		done = loginNotice(lang, "rejected")
	}
	if errors.Is(err, authcontract.ErrLoginNotFound) {
		_ = a.edit(ctx, cb.Message.Chat.ID, cb.Message.MessageID, loginNotice(lang, "expired"))
		return a.answer(ctx, cb.ID, loginToast(lang, "expired"))
	}
	if errors.Is(err, authcontract.ErrLoginClosed) {
		return a.answer(ctx, cb.ID, loginToast(lang, "closed"))
	}
	if err != nil {
		return err
	}
	if err = a.edit(ctx, cb.Message.Chat.ID, cb.Message.MessageID, done); err != nil {
		return err
	}
	return a.answer(ctx, cb.ID, "")
}

func (a *App) send(ctx context.Context, chatID int64, text string, markup any) error {
	err := a.client.SendMessage(ctx, chatID, text, markup)
	return a.ignorePermanent(err, "message not delivered")
}

func (a *App) edit(ctx context.Context, chatID, messageID int64, text string) error {
	return a.ignorePermanent(a.client.EditMessage(ctx, chatID, messageID, text), "login message not updated")
}

func (a *App) answer(ctx context.Context, id, text string) error {
	return a.ignorePermanent(a.client.AnswerCallback(ctx, id, text), "callback not answered")
}

func (a *App) ignorePermanent(err error, message string) error {
	if err == nil {
		return nil
	}
	var api *APIError
	if errors.As(err, &api) && !api.Temporary() {
		if a.log != nil {
			a.log.Warn(message, "error", err)
		}
		return nil
	}
	return err
}

func telegramProfile(from *User) contract.TelegramProfile {
	if from == nil {
		return contract.TelegramProfile{}
	}
	return contract.TelegramProfile{
		TelegramID:   from.ID,
		IsBot:        from.IsBot,
		FirstName:    from.FirstName,
		LastName:     from.LastName,
		Username:     from.Username,
		LanguageCode: from.LanguageCode,
		IsPremium:    from.IsPremium,
	}
}

func parseLoginCallback(data string) (approve bool, id string, ok bool) {
	prefix, id, found := strings.Cut(data, ":")
	if !found || !authcontract.ValidLoginID(id) {
		return false, "", false
	}
	switch prefix {
	case "y":
		return true, id, true
	case "n":
		return false, id, true
	default:
		return false, "", false
	}
}

func loginPrompt(browser, device, lang, id string) (string, inlineKeyboard) {
	ru := isRussian(lang)
	browser = displayLabel(browser, ru, "browser")
	device = displayLabel(device, ru, "device")
	yes, no := "Confirm", "Decline"
	text := fmt.Sprintf("Confirm this login.\n\nBrowser: %s\nDevice: %s\n\nIf this wasn't you, tap Decline.", browser, device)
	if ru {
		yes, no = "Подтвердить", "Отклонить"
		text = fmt.Sprintf("Подтвердите вход.\n\nБраузер: %s\nУстройство: %s\n\nЕсли это не вы, нажмите «Отклонить».", browser, device)
	}
	return text, inlineKeyboard{InlineKeyboard: [][]inlineButton{{
		{Text: yes, Data: "y:" + id},
		{Text: no, Data: "n:" + id},
	}}}
}

func loginNotice(lang, kind string) string {
	ru := isRussian(lang)
	switch kind {
	case "confirmed":
		if ru {
			return "Вход подтверждён. Вернитесь в браузер."
		}
		return "Login confirmed. Return to the browser."
	case "denied":
		if ru {
			return "Вход отклонён. Запросите новую ссылку в браузере."
		}
		return "Login declined. Request a new link in the browser."
	case "rejected":
		if ru {
			return "Вход отклонён."
		}
		return "Login declined."
	default:
		if ru {
			return "Ссылка для входа устарела. Запросите новую в браузере."
		}
		return "This login link has expired. Request a new one in the browser."
	}
}

func loginToast(lang, kind string) string {
	ru := isRussian(lang)
	switch kind {
	case "closed":
		if ru {
			return "Этот запрос уже закрыт"
		}
		return "This request is already closed"
	default:
		if ru {
			return "Ссылка устарела"
		}
		return "This link has expired"
	}
}

func displayLabel(value string, ru bool, kind string) string {
	value = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value))
	if utf8.RuneCountInString(value) > 64 {
		value = string([]rune(value)[:64])
	}
	if value != "" && value != "Unknown" {
		return value
	}
	if kind == "browser" {
		if ru {
			return "Неизвестный браузер"
		}
		return "Unknown browser"
	}
	if ru {
		return "Неизвестное устройство"
	}
	return "Unknown device"
}
