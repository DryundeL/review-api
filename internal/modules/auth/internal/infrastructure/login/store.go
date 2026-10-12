package login

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/application/ports"
)

// Store is the Redis login challenge. Keys expire on their own.
type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func loginKey(id string) string {
	return "auth:login:" + id
}

var createScript = redis.NewScript(`
redis.call('HSET', KEYS[1], 'browser', ARGV[1], 'device', ARGV[2], 'status', ARGV[3])
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
return 1
`)

var resolveScript = redis.NewScript(`
local status = redis.call('HGET', KEYS[1], 'status')
if not status then
  return 0
end
if status == 'pending' then
  redis.call('HSET', KEYS[1], 'status', ARGV[1], 'telegram_id', ARGV[2])
  redis.call('EXPIRE', KEYS[1], tonumber(ARGV[3]))
  return 1
end
if ARGV[1] == 'confirmed' and status == 'confirmed' and redis.call('HGET', KEYS[1], 'telegram_id') == ARGV[2] then
  return 1
end
if ARGV[1] == 'denied' and status == 'denied' then
  return 1
end
return 2
`)

var tokenScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'status') ~= 'confirmed' then
  return ''
end
local token = redis.call('HGET', KEYS[1], 'token')
if token and token ~= '' then
  return token
end
redis.call('HSET', KEYS[1], 'token', ARGV[1])
return ARGV[1]
`)

func (s *Store) Create(ctx context.Context, id string, rec ports.LoginRecord, ttl time.Duration) error {
	if s == nil || s.rdb == nil || !contract.ValidLoginID(id) {
		return contract.ErrLoginNotFound
	}
	err := createScript.Run(ctx, s.rdb, []string{loginKey(id)},
		rec.Browser, rec.Device, rec.Status, ttlSeconds(ttl),
	).Err()
	if err != nil {
		return fmt.Errorf("save login: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (ports.LoginRecord, error) {
	if s == nil || s.rdb == nil || !contract.ValidLoginID(id) {
		return ports.LoginRecord{}, contract.ErrLoginNotFound
	}
	vals, err := s.rdb.HGetAll(ctx, loginKey(id)).Result()
	if err != nil {
		return ports.LoginRecord{}, fmt.Errorf("read login: %w", err)
	}
	if vals["status"] == "" {
		return ports.LoginRecord{}, contract.ErrLoginNotFound
	}
	var telegramID int64
	if vals["telegram_id"] != "" {
		telegramID, err = strconv.ParseInt(vals["telegram_id"], 10, 64)
		if err != nil {
			return ports.LoginRecord{}, contract.ErrLoginNotFound
		}
	}
	return ports.LoginRecord{
		Browser:    vals["browser"],
		Device:     vals["device"],
		Status:     vals["status"],
		Token:      vals["token"],
		TelegramID: telegramID,
	}, nil
}

func (s *Store) Resolve(ctx context.Context, id, status string, telegramID int64, ttl time.Duration) error {
	if s == nil || s.rdb == nil || !contract.ValidLoginID(id) {
		return contract.ErrLoginNotFound
	}
	code, err := resolveScript.Run(ctx, s.rdb, []string{loginKey(id)},
		status, strconv.FormatInt(telegramID, 10), ttlSeconds(ttl),
	).Int64()
	if err != nil {
		return fmt.Errorf("resolve login: %w", err)
	}
	switch code {
	case 1:
		return nil
	case 2:
		return contract.ErrLoginClosed
	default:
		return contract.ErrLoginNotFound
	}
}

func (s *Store) SetToken(ctx context.Context, id, token string) (string, error) {
	if s == nil || s.rdb == nil || !contract.ValidLoginID(id) || token == "" {
		return "", contract.ErrLoginNotFound
	}
	saved, err := tokenScript.Run(ctx, s.rdb, []string{loginKey(id)}, token).Text()
	if err != nil {
		return "", fmt.Errorf("save login token: %w", err)
	}
	return saved, nil
}

func ttlSeconds(ttl time.Duration) string {
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
