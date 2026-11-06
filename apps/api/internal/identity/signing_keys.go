package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/rediscache"
)

// envKeyID is the fixed kid for the bootstrap key sourced from the
// JWT_SECRET env var — every token ever issued before this feature
// existed, and every token issued after it until the first rotation, use
// this kid.
const envKeyID = "env"

// signingKeySet is the Redis-persisted record: every key the store will
// currently accept for verification, plus which one newly-issued tokens
// are signed with. Persisted (not in-memory-only) so a rotation survives
// a process restart — platform-security-hardening task 1.9.
type signingKeySet struct {
	Current string            `json:"current"`
	Keys    map[string]string `json:"keys"` // kid -> base64(secret)
}

const signingKeysRedisKey = "jwt:signing_keys"

// SigningKeyStore holds the set of HMAC secrets TokenIssuer may sign or
// verify with, identified by kid (key ID), so a platform-wide "end all
// sessions" can rotate the signing key — and, unlike the env-sourced
// JWT_SECRET, without a deploy. Bootstraps itself from envSecret on
// first use so the feature is a no-op until a rotation actually happens.
type SigningKeyStore struct {
	redis     *rediscache.Client
	envSecret []byte

	mu        sync.RWMutex
	cached    *signingKeySet
	cachedAt  time.Time
	cacheTTL  time.Duration
}

// NewSigningKeyStore builds a store whose bootstrap key (kid "env") is
// envSecret — the existing JWT_SECRET. cacheTTL bounds how long a
// rotation can take to be picked up by a process that already had the
// old key set cached; 0 disables caching (every call hits Redis).
func NewSigningKeyStore(redis *rediscache.Client, envSecret string, cacheTTL time.Duration) *SigningKeyStore {
	return &SigningKeyStore{redis: redis, envSecret: []byte(envSecret), cacheTTL: cacheTTL}
}

func (s *SigningKeyStore) load(ctx context.Context) (*signingKeySet, error) {
	s.mu.RLock()
	if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
		cached := s.cached
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	raw, ok, err := s.redis.Get(ctx, signingKeysRedisKey)
	if err != nil {
		return nil, fmt.Errorf("load signing keys: %w", err)
	}
	var set signingKeySet
	if !ok {
		// Never rotated — bootstrap in memory only; Rotate is what
		// actually persists a signingKeySet to Redis for the first time.
		set = signingKeySet{
			Current: envKeyID,
			Keys:    map[string]string{envKeyID: base64.StdEncoding.EncodeToString(s.envSecret)},
		}
	} else if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf("unmarshal signing keys: %w", err)
	}

	s.mu.Lock()
	s.cached = &set
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return &set, nil
}

func (s *SigningKeyStore) save(ctx context.Context, set *signingKeySet) error {
	raw, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("marshal signing keys: %w", err)
	}
	if err := s.redis.Set(ctx, signingKeysRedisKey, raw, 0); err != nil {
		return fmt.Errorf("save signing keys: %w", err)
	}
	s.mu.Lock()
	s.cached = set
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return nil
}

// Current returns the kid and secret newly-issued tokens should be
// signed with.
func (s *SigningKeyStore) Current(ctx context.Context) (kid string, secret []byte, err error) {
	set, err := s.load(ctx)
	if err != nil {
		return "", nil, err
	}
	b64, ok := set.Keys[set.Current]
	if !ok {
		return "", nil, fmt.Errorf("current kid %q has no stored key", set.Current)
	}
	secret, err = base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", nil, fmt.Errorf("decode current key: %w", err)
	}
	return set.Current, secret, nil
}

// Lookup returns the secret for kid, for verifying a token's signature.
// ok is false if kid is unknown — either never issued, or purged by a
// prior RotateAndPurge.
func (s *SigningKeyStore) Lookup(ctx context.Context, kid string) (secret []byte, ok bool, err error) {
	set, err := s.load(ctx)
	if err != nil {
		return nil, false, err
	}
	b64, found := set.Keys[kid]
	if !found {
		return nil, false, nil
	}
	secret, err = base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, false, fmt.Errorf("decode key %q: %w", kid, err)
	}
	return secret, true, nil
}

// Rotate generates a fresh random key, makes it the signing key for
// every new token from now on, and keeps every previously-valid key
// verifiable (a graceful rotation — in-flight tokens signed moments ago
// keep working rather than being abruptly invalidated).
func (s *SigningKeyStore) Rotate(ctx context.Context) (newKid string, err error) {
	set, err := s.load(ctx)
	if err != nil {
		return "", err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate signing key: %w", err)
	}
	newKid = uuid.NewString()
	next := signingKeySet{Current: newKid, Keys: map[string]string{}}
	for k, v := range set.Keys {
		next.Keys[k] = v
	}
	next.Keys[newKid] = base64.StdEncoding.EncodeToString(secret)
	if err := s.save(ctx, &next); err != nil {
		return "", err
	}
	return newKid, nil
}

// RotateAndPurge rotates to a fresh key the same way Rotate does, but
// then drops every other key from the valid set — so every token signed
// before this call, regardless of which key it used, immediately fails
// verification. This is the platform-wide "end all sessions" button's
// extra layer (task 1.6/1.9): even a token whose `sessions` row was
// somehow missed by RevokeAllSessions stops verifying at all.
func (s *SigningKeyStore) RotateAndPurge(ctx context.Context) (newKid string, err error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate signing key: %w", err)
	}
	newKid = uuid.NewString()
	set := signingKeySet{
		Current: newKid,
		Keys:    map[string]string{newKid: base64.StdEncoding.EncodeToString(secret)},
	}
	if err := s.save(ctx, &set); err != nil {
		return "", err
	}
	return newKid, nil
}
