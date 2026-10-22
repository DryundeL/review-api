package authcode

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	TTL         = 10 * time.Minute
	Cooldown    = time.Minute
	MaxAttempts = 5
)

var (
	ErrInvalid     = errors.New("invalid code")
	ErrCooldown    = errors.New("code recently sent")
	ErrUnavailable = errors.New("verification unavailable")
)

type Store struct {
	rdb    *redis.Client
	secret []byte
	prefix string
}

func New(rdb *redis.Client, secret []byte, prefix string) *Store {
	return &Store{rdb: rdb, secret: secret, prefix: prefix}
}

func (s *Store) Issue(ctx context.Context, subject, value string) (string, error) {
	if s == nil || s.rdb == nil || len(s.secret) < 32 || subject == "" || value == "" {
		return "", ErrUnavailable
	}
	code, err := generateCode()
	if err != nil {
		return "", err
	}
	key := s.key(subject)
	ok, err := issueScript.Run(ctx, s.rdb, []string{key}, s.digest(code), value, int(TTL.Seconds()), int(Cooldown.Seconds())).Int()
	if err != nil {
		return "", fmt.Errorf("issue verification code: %w", err)
	}
	if ok == 0 {
		return "", ErrCooldown
	}
	return code, nil
}

func (s *Store) Verify(ctx context.Context, subject, code string) (string, error) {
	if s == nil || s.rdb == nil || len(s.secret) < 32 || subject == "" || len(code) != 6 {
		return "", ErrInvalid
	}
	result, err := verifyScript.Run(ctx, s.rdb, []string{s.key(subject)}, s.digest(code), MaxAttempts).Text()
	if err != nil {
		return "", fmt.Errorf("verify code: %w", err)
	}
	if result == "" {
		return "", ErrInvalid
	}
	return result, nil
}

func (s *Store) Cancel(ctx context.Context, subject string) {
	if s != nil && s.rdb != nil && subject != "" {
		_ = s.rdb.Del(ctx, s.key(subject), s.key(subject)+":cooldown").Err()
	}
}

func (s *Store) key(subject string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(subject))
	return s.prefix + ":" + hex.EncodeToString(mac.Sum(nil))
}

func (s *Store) digest(code string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func generateCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

var issueScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1] .. ':cooldown') == 1 then return 0 end
redis.call('HSET', KEYS[1], 'digest', ARGV[1], 'value', ARGV[2], 'attempts', 0)
redis.call('EXPIRE', KEYS[1], ARGV[3])
redis.call('SET', KEYS[1] .. ':cooldown', 1, 'EX', ARGV[4])
return 1
`)

var verifyScript = redis.NewScript(`
local digest = redis.call('HGET', KEYS[1], 'digest')
if not digest then return '' end
local value = redis.call('HGET', KEYS[1], 'value')
local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
if attempts >= tonumber(ARGV[2]) then
  redis.call('DEL', KEYS[1])
end
if digest ~= ARGV[1] then return '' end
redis.call('DEL', KEYS[1])
return value
`)
