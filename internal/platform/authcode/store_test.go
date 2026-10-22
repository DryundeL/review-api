package authcode

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestVerifyDeletesChallengeAfterFifthInvalidAttempt(t *testing.T) {
	t.Parallel()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := New(client, []byte("01234567890123456789012345678901"), "test:code")
	ctx := context.Background()

	code, err := store.Issue(ctx, "user-42", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	wrong := "000000"
	if code == wrong {
		wrong = "999999"
	}
	for range MaxAttempts {
		if _, err := store.Verify(ctx, "user-42", wrong); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid attempt: %v", err)
		}
	}
	if _, err := store.Verify(ctx, "user-42", code); !errors.Is(err, ErrInvalid) {
		t.Fatalf("challenge survived fifth attempt: %v", err)
	}
}
