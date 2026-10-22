package application

import (
	"context"
	"testing"
	"time"

	"review-api/internal/modules/auth/internal/domain"
)

type emailResolverStub struct {
	email string
	id    int64
	found bool
}

func (s emailResolverStub) ResolveLoginEmail(context.Context, string) (string, int64, bool, error) {
	return s.email, s.id, s.found, nil
}

type emailChallengeStub struct {
	code string
}

func (s *emailChallengeStub) Issue(context.Context, string, string) (string, error) {
	return s.code, nil
}

func (s *emailChallengeStub) Verify(_ context.Context, _ string, code string) (string, error) {
	if code != s.code {
		return "", ErrEmailCodeInvalid
	}
	return "user@example.com", nil
}

func (*emailChallengeStub) Cancel(context.Context, string) {}

type emailSenderStub struct{}

func (emailSenderStub) SendCode(context.Context, string, string, string) error { return nil }

func TestEmailLoginConfirmIssuesJWTForKnownEmail(t *testing.T) {
	t.Parallel()
	secret := []byte("01234567890123456789012345678901")
	service := NewEmailLoginService(
		emailResolverStub{email: "user@example.com", id: 42, found: true},
		&emailChallengeStub{code: "123456"},
		emailSenderStub{},
		secret,
		time.Hour,
	)

	token, err := service.Confirm(context.Background(), "USER@example.com", "123456")
	if err != nil {
		t.Fatal(err)
	}
	id, err := domain.ParseJWT(secret, token, time.Now())
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}

func TestEmailLoginConfirmHidesUnknownEmail(t *testing.T) {
	t.Parallel()
	service := NewEmailLoginService(
		emailResolverStub{email: "missing@example.com", found: false},
		&emailChallengeStub{code: "123456"},
		emailSenderStub{},
		[]byte("01234567890123456789012345678901"),
		time.Hour,
	)

	if _, err := service.Confirm(context.Background(), "missing@example.com", "123456"); err != ErrEmailCodeInvalid {
		t.Fatalf("err=%v", err)
	}
}
