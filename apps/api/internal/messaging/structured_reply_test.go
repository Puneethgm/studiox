package messaging

import (
	"encoding/json"
	"testing"
)

// TestStructuredReply_JSONRoundTrip covers task 3.6: StructuredReply
// marshals/unmarshals every field correctly — the shape CompleteStructured
// actually parses a provider's response into.
func TestStructuredReply_JSONRoundTrip(t *testing.T) {
	want := StructuredReply{
		Reply:     "Sure, S$99/month!",
		Action:    "answer",
		Language:  "en",
		Intent:    "pricing",
		Sentiment: "positive",
		FactsUsed: []string{"Monthly plan"},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got StructuredReply
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Reply != want.Reply || got.Action != want.Action ||
		got.Language != want.Language || got.Intent != want.Intent || got.Sentiment != want.Sentiment ||
		len(got.FactsUsed) != len(want.FactsUsed) || got.FactsUsed[0] != want.FactsUsed[0] {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

// TestStructuredReplyJSONSchema_RequiredFieldsHaveProperties confirms
// the schema is internally consistent: every field listed as "required"
// actually has a matching entry under "properties" — a schema that
// required a field it never defined would confuse (or be rejected by) a
// provider.
func TestStructuredReplyJSONSchema_RequiredFieldsHaveProperties(t *testing.T) {
	props, ok := StructuredReplyJSONSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema has no \"properties\" map")
	}
	required, ok := StructuredReplyJSONSchema["required"].([]string)
	if !ok {
		t.Fatal("schema has no \"required\" string slice")
	}
	if len(required) == 0 {
		t.Fatal("schema's required list is empty")
	}
	for _, field := range required {
		if _, ok := props[field]; !ok {
			t.Errorf("required field %q has no matching entry in properties", field)
		}
	}
}
