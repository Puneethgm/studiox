package llmgateway

import (
	"context"
	"testing"

	"github.com/projectx/api/internal/integrations/bifrost"
)

type fixtureStructuredReply struct {
	Reply    string `json:"reply"`
	Language string `json:"language"`
}

// TestCompleteStructured_ValidJSON_ParsesIntoStruct covers task 3.1: a
// provider response that's valid JSON for T parses correctly and the
// request it sent carries the response-format schema.
func TestCompleteStructured_ValidJSON_ParsesIntoStruct(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: `{"reply":"Hi there!","language":"en"}`}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_structured_success")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 60000)

	got, ref, err := CompleteStructured[fixtureStructuredReply](context.Background(), client, feature, studioID, "hi", "reply_schema", map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("CompleteStructured: %v", err)
	}
	if got.Reply != "Hi there!" || got.Language != "en" {
		t.Errorf("got = %+v, want {Reply:\"Hi there!\" Language:\"en\"}", got)
	}
	if ref == "" {
		t.Error("expected a non-empty sourceRef on success")
	}

	if len(completer.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(completer.calls))
	}
	if completer.calls[0].ResponseFormat == nil {
		t.Error("request sent to the provider did not carry a ResponseFormat")
	} else if completer.calls[0].ResponseFormat.Name != "reply_schema" {
		t.Errorf("ResponseFormat.Name = %q, want %q", completer.calls[0].ResponseFormat.Name, "reply_schema")
	}
}

// TestCompleteStructured_MalformedJSON_FallsBackToNextProvider covers
// the "a provider's structured output doesn't actually parse" case —
// design.md's gateway contract tries the next provider rather than
// surfacing a hard failure from one bad response.
func TestCompleteStructured_MalformedJSON_FallsBackToNextProvider(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: `not valid json at all`}},
		{reply: bifrost.Reply{Text: `{"reply":"from the fallback","language":"en"}`}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai", "gemini")
	feature := Feature("test_structured_fallback")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai", "gemini"}}, 60000)

	got, _, err := CompleteStructured[fixtureStructuredReply](context.Background(), client, feature, studioID, "hi", "reply_schema", map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("CompleteStructured: %v", err)
	}
	if got.Reply != "from the fallback" {
		t.Errorf("got.Reply = %q, want fallback provider's reply", got.Reply)
	}
}

// TestCompleteStructured_AllProvidersMalformed_ReturnsError covers the
// "never silently return a zero value" contract — unlike Complete, a
// structured caller must get a real error, not an empty-but-no-error
// zero value, when nothing parseable came back.
func TestCompleteStructured_AllProvidersMalformed_ReturnsError(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: `not json`}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_structured_all_malformed")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 60000)

	_, ref, err := CompleteStructured[fixtureStructuredReply](context.Background(), client, feature, studioID, "hi", "reply_schema", map[string]any{"type": "object"})
	if err == nil {
		t.Fatal("expected a non-nil error when no provider returns parseable JSON")
	}
	if ref != "" {
		t.Errorf("sourceRef = %q, want empty on failure", ref)
	}
}

// TestCompleteStructured_NoRouteConfigured covers the ErrNoRoute
// passthrough — same contract as Complete's equivalent test.
func TestCompleteStructured_NoRouteConfigured(t *testing.T) {
	completer := &fakeCompleter{}
	client, _, studioID := setupGatewayTest(t, completer)

	_, _, err := CompleteStructured[fixtureStructuredReply](context.Background(), client, Feature("test_structured_no_route_"+studioID.String()), studioID, "hi", "reply_schema", map[string]any{"type": "object"})
	if err == nil {
		t.Fatal("expected an error for an unconfigured feature route")
	}
}
