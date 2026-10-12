package contract

import (
	"context"
	"errors"
)

var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrIdentityMissing = errors.New("telegram identity missing")
)

type TelegramIdentity struct {
	TelegramID   int64
	IsBot        bool
	Username     string
	LanguageCode string
	PhotoURL     string
	FirstName    string
	LastName     string
	IsPremium    bool
}

type ctxKey struct{}

func WithIdentity(ctx context.Context, id TelegramIdentity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func IdentityFromContext(ctx context.Context) (TelegramIdentity, error) {
	id, ok := ctx.Value(ctxKey{}).(TelegramIdentity)
	if !ok || id.TelegramID == 0 {
		return TelegramIdentity{}, ErrIdentityMissing
	}
	return id, nil
}
