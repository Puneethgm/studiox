// Package rediscache is a thin wrapper around go-redis for byte-blob caching
// with a TTL. It carries no domain knowledge (cache keys, value encoding) —
// that lives with each caller.
package rediscache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps a go-redis client. The connection is lazy (go-redis dials on
// first command), so New never fails just because Redis isn't reachable yet
// — callers degrade per-call instead.
type Client struct {
	rdb *redis.Client
}

// New builds a Client for the Redis instance at addr (host:port), using
// password (required — see docker-compose.yml's redis service, which
// refuses to start without one) and the given logical DB index. Discrete
// fields, not a redis://user:pass@host/db URL, so a password containing
// URL-special characters (@, :, /, #, …) can never be mis-parsed.
func New(addr, password string, db int) *Client {
	return &Client{rdb: redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})}
}

// Get returns the stored value for key. The second return is false on a
// miss (key absent or expired); it is also false, alongside a non-nil error,
// on a backend failure — callers that want to fall through to the origin on
// error, not just on a genuine miss, should check the error.
func (c *Client) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

// Set stores value under key with the given TTL.
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Del removes key. Deleting an absent key is a no-op, not an error.
func (c *Client) Del(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// IncrWithTTL atomically increments key (starting from 0 if absent) and
// returns the new value. ttl is applied only the first time key is
// created (ExpireNX — "set expiry only if none exists"), so repeated
// calls within the same window extend the counter's value but not its
// expiry — the window is anchored to the first increment, not sliding.
// Used for counters like a lockout's failed-attempt tally (platform-
// security-hardening task 2.4/4.3).
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if err := c.rdb.ExpireNX(ctx, key, ttl).Err(); err != nil {
		return 0, err
	}
	return n, nil
}

// Close releases the underlying connection pool.
func (c *Client) Close() error {
	return c.rdb.Close()
}
