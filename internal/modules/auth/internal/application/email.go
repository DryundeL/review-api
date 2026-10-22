package application

import (
	"context"
	"errors"
	"time"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/domain"
)

var (
	ErrEmailCodeInvalid = errors.New("invalid verification code")
	ErrEmailCooldown    = errors.New("verification code recently sent")
	ErrEmailUnavailable = errors.New("email delivery unavailable")
)

type EmailResolver interface {
	ResolveLoginEmail(ctx context.Context, raw string) (normalized string, telegramID int64, found bool, err error)
}

type EmailChallenge interface {
	Issue(ctx context.Context, subject, value string) (string, error)
	Verify(ctx context.Context, subject, code string) (string, error)
	Cancel(ctx context.Context, subject string)
}

type EmailSender interface {
	SendCode(ctx context.Context, to, code, purpose string) error
}

type EmailLoginService struct {
	resolver  EmailResolver
	challenge EmailChallenge
	sender    EmailSender
	secret    []byte
	maxAge    time.Duration
}

func NewEmailLoginService(resolver EmailResolver, challenge EmailChallenge, sender EmailSender, secret []byte, maxAge time.Duration) *EmailLoginService {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &EmailLoginService{resolver: resolver, challenge: challenge, sender: sender, secret: secret, maxAge: maxAge}
}

func (s *EmailLoginService) Start(ctx context.Context, rawEmail string) error {
	if s == nil || s.resolver == nil || s.challenge == nil || s.sender == nil {
		return errors.New("email login unavailable")
	}
	email, _, found, err := s.resolver.ResolveLoginEmail(ctx, rawEmail)
	if err != nil {
		if errors.Is(err, contract.ErrInvalidEmail) {
			return contract.ErrInvalidEmail
		}
		return err
	}
	code, err := s.challenge.Issue(ctx, email, email)
	if err != nil {
		if errors.Is(err, ErrEmailCooldown) {
			return err
		}
		return ErrEmailUnavailable
	}
	if !found {
		return nil
	}
	if err := s.sender.SendCode(ctx, email, code, "login"); err != nil {
		s.challenge.Cancel(ctx, email)
		return ErrEmailUnavailable
	}
	return nil
}

func (s *EmailLoginService) Confirm(ctx context.Context, rawEmail, code string) (string, error) {
	if s == nil || s.resolver == nil || s.challenge == nil {
		return "", ErrEmailCodeInvalid
	}
	email, telegramID, found, err := s.resolver.ResolveLoginEmail(ctx, rawEmail)
	if err != nil || !found {
		return "", ErrEmailCodeInvalid
	}
	if _, err := s.challenge.Verify(ctx, email, code); err != nil {
		return "", ErrEmailCodeInvalid
	}
	token, err := domain.SignJWT(s.secret, telegramID, time.Now().Add(s.maxAge))
	if err != nil {
		return "", err
	}
	return token, nil
}
