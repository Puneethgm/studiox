package llmgateway

import (
	"context"
	"testing"

	"github.com/projectx/api/internal/integrations/bifrost"
)

// TestCompleteStructured_ShortReply_NotRejectedWhenMinUsableLengthIsZero
// covers task 3.4: with the customer_reply route's min_usable_length set
// to 0 (this task's migration), a short-but-schema-valid reply (e.g.
// "Yes, 6pm works!") is not rejected for being short — the old
// ">=15 chars" heuristic no longer applies once a feature's route
// disables it.
func TestCompleteStructured_ShortReply_NotRejectedWhenMinUsableLengthIsZero(t *testing.T) {
	shortJSON := `{"reply":"Yes!","action":"answer","language":"en","intent":"booking","sentiment":"positive","facts_used":[]}`
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: shortJSON}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_short_reply_allowed")
	// min_usable_length: 0 — the disabled state this task's migration sets
	// for the real customer_reply feature.
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}, MinUsableLength: 0}, 60000)

	got, ref, err := CompleteStructured[fixtureStructuredReply](context.Background(), client, feature, studioID, "hi", "reply_schema", map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("CompleteStructured: %v", err)
	}
	if ref == "" {
		t.Fatal("a short-but-valid reply should not be rejected when min_usable_length is 0")
	}
	if got.Reply != "Yes!" {
		t.Errorf("got.Reply = %q, want %q", got.Reply, "Yes!")
	}
}

// TestCompleteStructured_ShortReply_RejectedWhenMinUsableLengthIsSet is
// the control case: a non-zero min_usable_length still rejects a short
// raw-text response (the mechanism itself — now config, not hardcoded Go
// — still works for features that want it).
func TestCompleteStructured_ShortReply_RejectedWhenMinUsableLengthIsSet(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: "ok"}}, // shorter than min_usable_length, and not valid JSON either
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_short_reply_rejected")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}, MinUsableLength: 15}, 60000)

	_, ref, err := CompleteStructured[fixtureStructuredReply](context.Background(), client, feature, studioID, "hi", "reply_schema", map[string]any{"type": "object"})
	if ref != "" {
		t.Error("a reply shorter than min_usable_length should not be treated as usable")
	}
	if err == nil {
		t.Error("expected an error when the only provider's reply is rejected as too short")
	}
}
