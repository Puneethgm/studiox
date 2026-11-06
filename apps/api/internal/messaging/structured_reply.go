package messaging

// StructuredReply is the customer_reply feature's structured LLM
// response (ai-reply-flow-llm-design doc §4.6/task 3.2): one LLM call
// returns the reply text plus classification metadata that previously
// needed separate intent/expand/rerank calls. Only Reply is consumed by
// the send path today (the pipeline downstream of the structured call —
// strip/guard/send — is unchanged from before this task and still
// operates on plain text); Action/Language/Intent/Sentiment/FactsUsed
// are logged for now and are the fields Group 9's ai_turns persistence
// and a later guards pass consume.
type StructuredReply struct {
	Reply     string   `json:"reply"`
	Action    string   `json:"action"`
	Language  string   `json:"language"`
	Intent    string   `json:"intent"`
	Sentiment string   `json:"sentiment"`
	FactsUsed []string `json:"facts_used"`
	// ToolCalls are live actions the model wants performed before
	// finalizing its reply (ai-reply-flow-llm-design task 7.2) —
	// availability-check/booking. Implemented via this same structured
	// call rather than Bifrost's separate native tool-calling wire
	// protocol: see executeToolCalls' doc comment for why.
	ToolCalls []ToolCallRequest `json:"tool_calls,omitempty"`
}

// ToolCallRequest is one live action the model is asking the pipeline to
// perform — validated and executed by executeToolCalls, never trusted
// as already having happened.
type ToolCallRequest struct {
	Tool      string         `json:"tool"` // "check_availability" | "create_booking"
	Arguments map[string]any `json:"arguments"`
}

// StructuredReplySchemaName identifies this schema to the provider
// (OpenAI-shaped response_format.json_schema.name).
const StructuredReplySchemaName = "customer_reply"

// StructuredReplyJSONSchema is a JSON Schema object (not a Go struct tag
// reflection — Bifrost forwards this to the provider as-is, see
// bifrost.ResponseFormatSchema's doc comment) describing StructuredReply.
// Property descriptions carry the semantic guidance a plain prompt
// instruction would otherwise need to spell out, since schema-constrained
// generation is commonly description-aware across providers.
var StructuredReplyJSONSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"reply": map[string]any{
			"type":        "string",
			"description": "The customer-facing reply text itself — exactly what should be sent, nothing else.",
		},
		"action": map[string]any{
			"type":        "string",
			"description": "What this reply is doing: \"answer\" (answered the question), \"greeting\", \"handoff\" (deferring to a human), \"booking_inquiry\", or \"clarify\" (asked a clarifying question).",
			"enum":        []string{"answer", "greeting", "handoff", "booking_inquiry", "clarify"},
		},
		"language": map[string]any{
			"type":        "string",
			"description": "The language the reply is written in, as a short code (e.g. \"en\", \"ms\", \"zh\") matching the customer's own message language.",
		},
		"intent": map[string]any{
			"type":        "string",
			"description": "A short label for what the customer was asking about (e.g. \"pricing\", \"schedule\", \"cancellation\", \"general\").",
		},
		"sentiment": map[string]any{
			"type":        "string",
			"description": "The customer's apparent sentiment in their message.",
			"enum":        []string{"positive", "neutral", "negative"},
		},
		"facts_used": map[string]any{
			"type":        "array",
			"description": "Short labels naming which facts from the context above (e.g. a specific plan name, offer name, or knowledge-base fact) were actually used to answer — empty if none were needed.",
			"items":       map[string]any{"type": "string"},
		},
		"tool_calls": map[string]any{
			"type":        "array",
			"description": "Live actions to perform before finalizing the reply — at most 2 rounds of this will ever be honored. Use \"check_availability\" (arguments: {\"from\": unix_seconds, \"to\": unix_seconds}) to look up real bookable class times, or \"create_booking\" (arguments: {\"user_id\": string, \"event_id\": string, from a prior check_availability result}) to actually book one. Leave empty if no live action is needed — most replies need none.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool": map[string]any{"type": "string", "enum": []string{"check_availability", "create_booking"}},
					"arguments": map[string]any{
						"type":        "object",
						"description": "check_availability: {\"from\", \"to\"} as unix-second strings. create_booking: {\"user_id\", \"event_id\"} as strings. Omit fields the chosen tool doesn't use.",
						"properties": map[string]any{
							"from":     map[string]any{"type": []string{"number", "null"}, "description": "check_availability only: range start, unix seconds. null if not used."},
							"to":       map[string]any{"type": []string{"number", "null"}, "description": "check_availability only: range end, unix seconds. null if not used."},
							"user_id":  map[string]any{"type": []string{"string", "null"}, "description": "create_booking only: the customer's Glofox member id. null if not used."},
							"event_id": map[string]any{"type": []string{"string", "null"}, "description": "create_booking only: the event id to book, from a prior check_availability result. null if not used."},
						},
						"required":             []string{"from", "to", "user_id", "event_id"},
						"additionalProperties": false,
					},
				},
				"required":             []string{"tool", "arguments"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"reply", "action", "language", "intent", "sentiment", "facts_used", "tool_calls"},
	"additionalProperties": false,
}
