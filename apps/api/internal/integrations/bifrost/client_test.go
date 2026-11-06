package bifrost

import (
	"context"
	"os"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

// These tests cover this package's own glue code (the account{} implementation
// and Complete's validation/request-shaping), not Bifrost's internals — Bifrost
// now runs in-process rather than behind an HTTP interface we could intercept,
// so there is no wire format of ours left to assert on; that's covered by
// Bifrost's own upstream test suite.

func TestNewReturnsNilWhenDisabled(t *testing.T) {
	t.Setenv("BIFROST_ENABLED", "false")
	if New() != nil {
		t.Error("New() should return nil when BIFROST_ENABLED=false")
	}
}

func TestNewReturnsClientByDefault(t *testing.T) {
	os.Unsetenv("BIFROST_ENABLED")
	c := New()
	if c == nil {
		t.Fatal("New() should return a non-nil client by default")
	}
	c.bf.Shutdown()
}

func TestCompleteNilReceiver(t *testing.T) {
	var c *Client
	if _, err := c.Complete(context.Background(), Request{Provider: ProviderGroq, Model: "x", DirectKey: "k", Prompt: "hi"}); err == nil {
		t.Error("expected error calling Complete on a nil client")
	}
}

func TestComplete_RequiresProviderAndModel(t *testing.T) {
	c := New()
	defer c.bf.Shutdown()
	if _, err := c.Complete(context.Background(), Request{DirectKey: "k", Prompt: "hi"}); err == nil {
		t.Error("expected error with no Provider/Model set")
	}
}

func TestComplete_RequiresDirectKey(t *testing.T) {
	c := New()
	defer c.bf.Shutdown()
	if _, err := c.Complete(context.Background(), Request{Provider: ProviderGroq, Model: "x", Prompt: "hi"}); err == nil {
		t.Error("expected error with no DirectKey set")
	}
}

func TestAccount_GetConfiguredProviders(t *testing.T) {
	providers, err := account{}.GetConfiguredProviders()
	if err != nil {
		t.Fatalf("GetConfiguredProviders: %v", err)
	}
	want := map[schemas.ModelProvider]bool{schemas.Groq: true, schemas.Gemini: true, schemas.Anthropic: true, schemas.OpenAI: true}
	if len(providers) != len(want) {
		t.Fatalf("got %d providers, want %d", len(providers), len(want))
	}
	for _, p := range providers {
		if !want[p] {
			t.Errorf("unexpected provider %q", p)
		}
	}
}

func TestAccount_GetKeysForProvider_RequiresContextKey(t *testing.T) {
	if _, err := (account{}).GetKeysForProvider(context.Background(), schemas.Groq); err == nil {
		t.Error("expected error when no direct key is set on the context")
	}
}

func TestAccount_GetKeysForProvider_UsesContextKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), directKeyContextKey{}, "studio-own-key")
	keys, err := (account{}).GetKeysForProvider(ctx, schemas.Groq)
	if err != nil {
		t.Fatalf("GetKeysForProvider: %v", err)
	}
	if len(keys) != 1 || keys[0].Value.Val != "studio-own-key" {
		t.Errorf("keys = %+v, want a single key with Value.Val = %q", keys, "studio-own-key")
	}
}
