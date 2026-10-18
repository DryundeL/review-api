package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/internal/application/ports"
	"review-api/internal/modules/notifications/internal/domain"
	userscontract "review-api/internal/modules/users/contract"
)

type MuteChecker interface {
	NotificationsMutedByUserID(ctx context.Context, userID uuid.UUID) (bool, error)
}

type UserLookup interface {
	UserRef(ctx context.Context, userID uuid.UUID) (userscontract.UserRef, error)
}

type BotSender interface {
	SendMessage(ctx context.Context, chatID int64, text string, markup any) error
}

type CreateNotificationHandler struct {
	prefs ports.PrefRepository
	inbox ports.InboxRepository
	users UserLookup
	mute  MuteChecker
	bot   BotSender
	log   *slog.Logger
	now   func() time.Time
}

func NewCreateNotificationHandler(
	prefs ports.PrefRepository,
	inbox ports.InboxRepository,
	users UserLookup,
	mute MuteChecker,
	bot BotSender,
	log *slog.Logger,
) *CreateNotificationHandler {
	if log == nil {
		log = slog.Default()
	}
	return &CreateNotificationHandler{
		prefs: prefs, inbox: inbox, users: users, mute: mute, bot: bot, log: log, now: time.Now,
	}
}

type CreateInput struct {
	RecipientID uuid.UUID
	ActorID     uuid.UUID
	TypeName    string
	ReviewID    *uuid.UUID
	ReviewTitle string
}

// Handle creates an inbox row when the type is enabled; optionally mirrors to Telegram.
// Type disabled → no-op. Global mute → inbox only, no bot.
func (h *CreateNotificationHandler) Handle(ctx context.Context, in CreateInput) error {
	if in.RecipientID == uuid.Nil || in.ActorID == uuid.Nil || in.TypeName == "" {
		return nil
	}
	if in.RecipientID == in.ActorID {
		return nil
	}
	enabled, err := h.prefs.TypeEnabled(ctx, in.RecipientID, in.TypeName)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	if !enabled {
		return nil
	}
	meta, err := h.prefs.TypeMeta(ctx, in.TypeName)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	actor, err := h.users.UserRef(ctx, in.ActorID)
	if err != nil {
		return fmt.Errorf("create notification: actor: %w", err)
	}
	body := domain.RenderBody(meta.Text, actor.DisplayName(), in.ReviewTitle)
	item := ports.Notification{
		ID:        uuid.New(),
		UserID:    in.RecipientID,
		TypeName:  meta.Name,
		Title:     meta.Title,
		Body:      body,
		ActorID:   &in.ActorID,
		ReviewID:  in.ReviewID,
		CreatedAt: h.now().UTC(),
	}
	if err := h.inbox.Insert(ctx, item); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	muted, err := h.mute.NotificationsMutedByUserID(ctx, in.RecipientID)
	if err != nil {
		h.log.WarnContext(ctx, "notification mute check failed", "error", err, "user_id", in.RecipientID)
		return nil
	}
	if muted || h.bot == nil {
		return nil
	}
	recipient, err := h.users.UserRef(ctx, in.RecipientID)
	if err != nil {
		h.log.WarnContext(ctx, "notification recipient lookup failed", "error", err)
		return nil
	}
	text := item.Title + "\n" + item.Body
	if err := h.bot.SendMessage(ctx, recipient.TelegramID, text, nil); err != nil {
		// Bot failure must not retry outbox / duplicate inbox rows.
		h.log.WarnContext(ctx, "notification bot send failed", "error", err, "telegram_id", recipient.TelegramID)
	}
	return nil
}
