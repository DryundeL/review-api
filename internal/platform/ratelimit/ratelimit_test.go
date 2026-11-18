package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return server, client
}

func TestHitSetsWindowEvenWhenRequestIsCanceled(t *testing.T) {
	t.Parallel()
	server, client := newClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n, err := hit(ctx, client, "rl:203.0.113.1", time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("hit = %d, %v", n, err)
	}
	if ttl := server.TTL("rl:203.0.113.1"); ttl != time.Minute {
		t.Fatalf("ttl = %v, want %v", ttl, time.Minute)
	}
}

func TestHitKeepsWindowAndRepairsKeyWithoutTTL(t *testing.T) {
	t.Parallel()
	server, client := newClient(t)
	ctx := context.Background()

	if _, err := hit(ctx, client, "rl:a", time.Minute); err != nil {
		t.Fatal(err)
	}
	server.FastForward(30 * time.Second)
	n, err := hit(ctx, client, "rl:a", time.Minute)
	if err != nil || n != 2 {
		t.Fatalf("hit = %d, %v", n, err)
	}
	if ttl := server.TTL("rl:a"); ttl != 30*time.Second {
		t.Fatalf("window was extended: ttl = %v", ttl)
	}

	if err := server.Set("rl:stuck", "467"); err != nil {
		t.Fatal(err)
	}
	if _, err := hit(ctx, client, "rl:stuck", time.Minute); err != nil {
		t.Fatal(err)
	}
	if ttl := server.TTL("rl:stuck"); ttl != time.Minute {
		t.Fatalf("stuck key ttl = %v, want %v", ttl, time.Minute)
	}
}
