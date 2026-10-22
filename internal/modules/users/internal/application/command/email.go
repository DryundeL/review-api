package command

import (
	"context"
	"errors"
	"strconv"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

var (
	ErrCodeInvalid      = errors.New("invalid verification code")
	ErrCooldown         = errors.New("verification code recently sent")
	ErrEmailUnavailable = errors.New("email delivery unavailable")
)

type EmailChallenge interface {
	Issue(ctx context.Context, subject, email string) (string, error)
	Verify(ctx context.Context, subject, code string) (string, error)
	Cancel(ctx context.Context, subject string)
}

type EmailSender interface {
	SendCode(ctx context.Context, to, code, purpose string) error
}

type EmailHandler struct {
	repo      ports.EmailRepository
	challenge EmailChallenge
	sender    EmailSender
}

func NewEmailHandler(repo ports.EmailRepository, challenge EmailChallenge, sender EmailSender) *EmailHandler {
	return &EmailHandler{repo: repo, challenge: challenge, sender: sender}
}

func (h *EmailHandler) Start(ctx context.Context, telegramID int64, rawEmail string) (bool, error) {
	if h == nil || h.repo == nil || h.challenge == nil || h.sender == nil || telegramID <= 0 {
		return false, errors.New("email verification unavailable")
	}
	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return false, err
	}
	owner, err := h.repo.FindTelegramIDByEmail(ctx, email)
	switch {
	case err == nil && owner == telegramID:
		return true, nil
	case err == nil:
		return false, contract.ErrEmailConflict
	case !errors.Is(err, contract.ErrNotFound):
		return false, err
	}
	subject := strconv.FormatInt(telegramID, 10)
	code, err := h.challenge.Issue(ctx, subject, email)
	if err != nil {
		if errors.Is(err, ErrCooldown) {
			return false, err
		}
		return false, ErrEmailUnavailable
	}
	if err := h.sender.SendCode(ctx, email, code, "bind"); err != nil {
		h.challenge.Cancel(ctx, subject)
		return false, ErrEmailUnavailable
	}
	return false, nil
}

func (h *EmailHandler) Confirm(ctx context.Context, telegramID int64, code string) error {
	if h == nil || h.repo == nil || h.challenge == nil || telegramID <= 0 {
		return ErrCodeInvalid
	}
	email, err := h.challenge.Verify(ctx, strconv.FormatInt(telegramID, 10), code)
	if err != nil {
		return ErrCodeInvalid
	}
	return h.repo.SetEmail(ctx, telegramID, email)
}
