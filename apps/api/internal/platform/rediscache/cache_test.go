package rediscache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// testClient connects to the real dev Redis (same .env-driven, skip-if-
// unconfigured pattern as the Postgres integration tests elsewhere in this
// repo — see internal/messaging/daily_limit_test.go). Skips on a missing
// REDIS_PASSWORD, not just a missing host/port, since the docker-compose
// Redis instance requires auth and there's no meaningful default for a
// secret.
func testClient(t *testing.T) *Client {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		t.Skip("Skipping integration test; REDIS_PASSWORD not set")
	}
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	c := New(fmt.Sprintf("%s:%s", host, port), password, 0)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestSetThenGet(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	key := "rediscache_test:set_then_get"
	t.Cleanup(func() { _, _ = c.rdb.Del(ctx, key).Result() })

	if err := c.Set(ctx, key, []byte("hello"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	val, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("expected a hit after Set")
	}
	if string(val) != "hello" {
		t.Errorf("got %q, want %q", val, "hello")
	}
}

func TestGetMiss(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	_, ok, err := c.Get(ctx, "rediscache_test:definitely_absent_key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Error("expected a miss for a key that was never set")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	key := "rediscache_test:ttl_expiry"
	t.Cleanup(func() { _, _ = c.rdb.Del(ctx, key).Result() })

	if err := c.Set(ctx, key, []byte("short-lived"), 50*time.Millisecond); err != nil {
		t.Fatalf("set: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	_, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Error("expected the entry to have expired")
	}
}
