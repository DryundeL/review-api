package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application/ports"
	"review-api/internal/modules/auth/internal/domain"
)

type memStore struct {
	recs map[string]ports.LoginRecord
}

func (m *memStore) Create(_ context.Context, id string, rec ports.LoginRecord, _ time.Duration) error {
	if m.recs == nil {
		m.recs = map[string]ports.LoginRecord{}
	}
	m.recs[id] = rec
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (ports.LoginRecord, error) {
	rec, ok := m.recs[id]
	if !ok {
		return ports.LoginRecord{}, contract.ErrLoginNotFound
	}
	return rec, nil
}

func (m *memStore) Resolve(_ context.Context, id, status string, telegramID int64, _ time.Duration) error {
	rec, ok := m.recs[id]
	if !ok {
		return contract.ErrLoginNotFound
	}
	if rec.Status == contract.LoginPending {
		rec.Status = status
		rec.TelegramID = telegramID
		m.recs[id] = rec
		return nil
	}
	if status == contract.LoginConfirmed && rec.Status == status && rec.TelegramID == telegramID {
		return nil
	}
	if status == contract.LoginDenied && rec.Status == status {
		return nil
	}
	return contract.ErrLoginClosed
}

func (m *memStore) SetToken(_ context.Context, id, token string) (string, error) {
	rec, ok := m.recs[id]
	if !ok || rec.Status != contract.LoginConfirmed {
		return "", nil
	}
	if rec.Token == "" {
		rec.Token = token
		m.recs[id] = rec
	}
	return rec.Token, nil
}

func TestLoginConfirmIssuesJWT(t *testing.T) {
	t.Parallel()
	secret := []byte(strings.Repeat("s", 32))
	svc := NewLoginService(&memStore{}, secret, "review_bot", time.Hour)
	const ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	started, err := svc.Start(context.Background(), ua)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(started.URL, "https://t.me/review_bot?start=login_") || started.ExpiresIn != 300 {
		t.Fatalf("started = %+v", started)
	}
	login, err := svc.Get(context.Background(), started.ID)
	if err != nil || login.Browser != "Chrome" || login.Device != "macOS" || login.Status != contract.LoginPending {
		t.Fatalf("login = %+v err=%v", login, err)
	}
	status, token, err := svc.Poll(context.Background(), started.ID)
	if err != nil || status != contract.LoginPending || token != "" {
		t.Fatalf("pending status=%s token=%q err=%v", status, token, err)
	}
	if err := svc.Confirm(context.Background(), started.ID, 42); err != nil {
		t.Fatal(err)
	}
	status, token, err = svc.Poll(context.Background(), started.ID)
	if err != nil || status != contract.LoginConfirmed || token == "" {
		t.Fatalf("confirmed status=%s token=%q err=%v", status, token, err)
	}
	id, err := domain.ParseJWT(secret, token, time.Now())
	if err != nil || id != 42 {
		t.Fatalf("jwt id=%d err=%v", id, err)
	}
	_, again, err := svc.Poll(context.Background(), started.ID)
	if err != nil || again != token {
		t.Fatalf("second poll token changed: %v", err)
	}
	if err := svc.Deny(context.Background(), started.ID); err != contract.ErrLoginClosed {
		t.Fatalf("deny after confirm: %v", err)
	}
}
