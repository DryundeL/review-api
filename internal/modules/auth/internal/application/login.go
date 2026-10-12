package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application/ports"
	"review-api/internal/modules/auth/internal/domain"
)

const loginTTL = 5 * time.Minute

var errLoginUnavailable = errors.New("login unavailable")

// StartedLogin is what the browser needs to open the bot and then poll.
type StartedLogin struct {
	ID        string
	URL       string
	ExpiresIn int
}

// LoginService starts browser logins and turns a confirmed one into a JWT.
type LoginService struct {
	store    ports.LoginStore
	secret   []byte
	username string
	maxAge   time.Duration
	now      func() time.Time
}

func NewLoginService(store ports.LoginStore, secret []byte, botUsername string, maxAge time.Duration) *LoginService {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &LoginService{
		store:    store,
		secret:   secret,
		username: botUsername,
		maxAge:   maxAge,
		now:      time.Now,
	}
}

func (s *LoginService) Start(ctx context.Context, userAgent string) (StartedLogin, error) {
	if s == nil || s.store == nil || !safeUsername(s.username) {
		return StartedLogin{}, errLoginUnavailable
	}
	id, err := newLoginID()
	if err != nil {
		return StartedLogin{}, err
	}
	browser, device := domain.ClientLabel(userAgent)
	err = s.store.Create(ctx, id, ports.LoginRecord{
		Browser: browser,
		Device:  device,
		Status:  contract.LoginPending,
	}, loginTTL)
	if err != nil {
		return StartedLogin{}, err
	}
	return StartedLogin{
		ID:        id,
		URL:       "https://t.me/" + s.username + "?start=login_" + id,
		ExpiresIn: int(loginTTL.Seconds()),
	}, nil
}

func (s *LoginService) Get(ctx context.Context, id string) (contract.Login, error) {
	rec, err := s.load(ctx, id)
	if err != nil {
		return contract.Login{}, err
	}
	return contract.Login{
		Browser:    rec.Browser,
		Device:     rec.Device,
		Status:     rec.Status,
		TelegramID: rec.TelegramID,
	}, nil
}

func (s *LoginService) Confirm(ctx context.Context, id string, telegramID int64) error {
	if s == nil || s.store == nil || !contract.ValidLoginID(id) || telegramID <= 0 {
		return contract.ErrLoginNotFound
	}
	return s.store.Resolve(ctx, id, contract.LoginConfirmed, telegramID, loginTTL)
}

func (s *LoginService) Deny(ctx context.Context, id string) error {
	if s == nil || s.store == nil || !contract.ValidLoginID(id) {
		return contract.ErrLoginNotFound
	}
	return s.store.Resolve(ctx, id, contract.LoginDenied, 0, loginTTL)
}

// Poll returns the status and, once confirmed, the same JWT on every call until expiry.
func (s *LoginService) Poll(ctx context.Context, id string) (string, string, error) {
	rec, err := s.load(ctx, id)
	if err != nil {
		return "", "", err
	}
	if rec.Status != contract.LoginConfirmed {
		return rec.Status, "", nil
	}
	if rec.TelegramID <= 0 {
		return "", "", contract.ErrLoginNotFound
	}
	if rec.Token != "" {
		return rec.Status, rec.Token, nil
	}
	token, err := domain.SignJWT(s.secret, rec.TelegramID, s.now().Add(s.maxAge))
	if err != nil {
		return "", "", err
	}
	saved, err := s.store.SetToken(ctx, id, token)
	if err != nil {
		return "", "", err
	}
	if saved == "" {
		return "", "", contract.ErrLoginNotFound
	}
	return contract.LoginConfirmed, saved, nil
}

func (s *LoginService) load(ctx context.Context, id string) (ports.LoginRecord, error) {
	if s == nil || s.store == nil || !contract.ValidLoginID(id) {
		return ports.LoginRecord{}, contract.ErrLoginNotFound
	}
	return s.store.Get(ctx, id)
}

func newLoginID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func safeUsername(name string) bool {
	if len(name) < 5 || len(name) > 32 {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && ((r >= '0' && r <= '9') || r == '_'):
		default:
			return false
		}
	}
	return true
}
