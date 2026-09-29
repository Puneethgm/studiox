package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/projectx/api/internal/platform/rediscache"
)

func TestAnswerCacheKey_DifferentStudiosDifferentKeys(t *testing.T) {
	studioA := uuid.New()
	studioB := uuid.New()
	keyA := answerCacheKey(studioA, "test_chat", "same prompt text")
	keyB := answerCacheKey(studioB, "test_chat", "same prompt text")
	if keyA == keyB {
		t.Fatalf("two different studios produced the same cache key: %q", keyA)
	}
}

func TestAnswerCacheKey_DifferentKindsDifferentKeys(t *testing.T) {
	studioID := uuid.New()
	keyA := answerCacheKey(studioID, "test_chat", "same prompt text")
	keyB := answerCacheKey(studioID, "incoming_reply", "same prompt text")
	if keyA == keyB {
		t.Fatalf("two different kinds for the same studio produced the same cache key: %q", keyA)
	}
}

func TestAnswerCache_NilCacheIsAlwaysMissAndSetIsNoop(t *testing.T) {
	var c *AnswerCache // nil — same as an unwired cache in tests/local dev
	ctx := context.Background()
	studioID := uuid.New()

	if text, source, ok := c.Get(ctx, studioID, "test_chat", "hello"); ok || text != "" || source != "" {
		t.Fatalf("nil cache Get: got (%q, %q, %v), want (\"\", \"\", false)", text, source, ok)
	}

	// Set on a nil cache must not panic.
	c.Set(ctx, studioID, "test_chat", "hello", "answer", "groq:model")
}

// testAnswerCache connects to the real dev Redis, same skip-if-unconfigured
// pattern as internal/platform/rediscache's own tests.
func testAnswerCache(t *testing.T) *AnswerCache {
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
	client := rediscache.New(fmt.Sprintf("%s:%s", host, port), password, 0)
	t.Cleanup(func() { _ = client.Close() })
	return NewAnswerCache(client, time.Minute)
}

func TestAnswerCache_SetThenGet(t *testing.T) {
	c := testAnswerCache(t)
	ctx := context.Background()
	studioID := uuid.New()

	if _, _, ok := c.Get(ctx, studioID, "test_chat", "what are your hours?"); ok {
		t.Fatal("expected a miss before Set")
	}

	c.Set(ctx, studioID, "test_chat", "what are your hours?", "We're open 6am-10pm daily.", "groq:llama-3.1-8b")

	text, source, ok := c.Get(ctx, studioID, "test_chat", "what are your hours?")
	if !ok {
		t.Fatal("expected a hit after Set")
	}
	if text != "We're open 6am-10pm daily." || source != "groq:llama-3.1-8b" {
		t.Errorf("got (%q, %q), want the values passed to Set", text, source)
	}
}

func TestAnswerCache_TwoStudiosSamePromptDoNotShareAnAnswer(t *testing.T) {
	c := testAnswerCache(t)
	ctx := context.Background()
	studioA := uuid.New()
	studioB := uuid.New()
	prompt := "what are your hours?"

	c.Set(ctx, studioA, "test_chat", prompt, "Studio A: 6am-10pm.", "groq:llama-3.1-8b")

	if text, _, ok := c.Get(ctx, studioB, "test_chat", prompt); ok {
		t.Errorf("studio B got studio A's cached answer: %q", text)
	}
}

func TestAnswerCache_SetIgnoresEmptyText(t *testing.T) {
	c := testAnswerCache(t)
	ctx := context.Background()
	studioID := uuid.New()

	// An empty answer (e.g. all providers failed) must never be cached —
	// otherwise the next identical query would replay the failure forever.
	c.Set(ctx, studioID, "test_chat", "unanswerable prompt", "", "")

	if _, _, ok := c.Get(ctx, studioID, "test_chat", "unanswerable prompt"); ok {
		t.Error("expected an empty answer to not be cached")
	}
}
