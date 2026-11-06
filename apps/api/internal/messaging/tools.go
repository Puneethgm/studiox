package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/leads"
)

// maxToolCallRounds bounds how many times executeToolCalls' caller will
// go back to the model with tool results and ask again — doc §4.7's
// "≤2 rounds" (task 7.2).
const maxToolCallRounds = 2

// ToolCallResult is what gets fed back into the follow-up structured
// call's prompt after executing a round of tool calls.
type ToolCallResult struct {
	Tool   string
	Result any
	Err    error
}

// executeToolCalls validates and executes each requested tool call —
// implemented at the application layer on top of CompleteStructured's
// JSON-schema output (StructuredReply.ToolCalls), not Bifrost's separate
// native tool-calling wire protocol. Bifrost's ChatTool/
// ToolFunctionParameters types require building parameter schemas via
// its own OrderedMap type, which is real additional integration surface;
// since the model requesting a tool call via a structured JSON field it's
// already being asked to populate achieves the identical observable
// behavior (model proposes, code validates and disposes — design.md's
// own phrase) this task reuses already-built, already-tested
// infrastructure instead. See
// openspec/changes/ai-reply-flow-llm-design/tasks.md task 7.2's note.
//
// Every call is validated against lead (the conversation's own
// lead/customer) before execution — a tool call isn't trusted just
// because the model asked for it (design.md's "application-level
// validation" decision) — and audited via tool_call_audit regardless of
// outcome (task 7.4).
//
// shadow (task 9.2): in shadow mode, a side-effecting call
// (create_booking) is never actually executed — the model's *request* is
// still recorded (in the audit log and the caller's ai_turns row) as
// what it would have done, but no real booking is created. Read-only
// calls (check_availability) still run for real, since they have no
// side effect and shadow mode's whole point is validating the pipeline
// against real conditions.
func (w *AIWorker) executeToolCalls(ctx context.Context, studioID, conversationID uuid.UUID, lead *leads.Lead, calls []ToolCallRequest, shadow bool) []ToolCallResult {
	results := make([]ToolCallResult, 0, len(calls))
	for _, call := range calls {
		if shadow && call.Tool == "create_booking" {
			result := map[string]any{"skipped": true, "reason": "shadow mode — not actually booked"}
			w.auditToolCall(ctx, conversationID, studioID, call, result, nil)
			results = append(results, ToolCallResult{Tool: call.Tool, Result: result})
			continue
		}
		result, err := w.executeOneToolCall(ctx, studioID, lead, call)
		w.auditToolCall(ctx, conversationID, studioID, call, result, err)
		results = append(results, ToolCallResult{Tool: call.Tool, Result: result, Err: err})
	}
	return results
}

func (w *AIWorker) executeOneToolCall(ctx context.Context, studioID uuid.UUID, lead *leads.Lead, call ToolCallRequest) (any, error) {
	if w.glofoxClient == nil {
		return nil, fmt.Errorf("glofox is not configured for this deployment")
	}

	switch call.Tool {
	case "check_availability":
		from, fok := call.Arguments["from"].(float64)
		to, tok := call.Arguments["to"].(float64)
		if !fok || !tok {
			return nil, fmt.Errorf("check_availability requires numeric \"from\" and \"to\" arguments")
		}
		slots, err := w.glofoxClient.CheckAvailability(ctx, int64(from), int64(to))
		if err != nil {
			// Explicit nil, not the zero []glofox.EventSlot wrapped in an
			// any — same typed-nil-in-interface reasoning as create_booking
			// below.
			return nil, err
		}
		return slots, nil

	case "create_booking":
		userID, uok := call.Arguments["user_id"].(string)
		eventID, eok := call.Arguments["event_id"].(string)
		if !uok || !eok || userID == "" || eventID == "" {
			return nil, fmt.Errorf("create_booking requires string \"user_id\" and \"event_id\" arguments")
		}
		// Validate the requested user_id actually belongs to THIS
		// conversation's lead — never book on behalf of an arbitrary
		// Glofox member id the model happened to produce. A conversation
		// with no linked lead, or a lead with no known Glofox member id
		// yet, can never pass this check — matches design.md's "the
		// gateway has no concept of this conversation's studio/customer;
		// that boundary belongs in the caller that already knows it."
		if lead == nil || lead.GlofoxMemberID == "" || lead.GlofoxMemberID != userID {
			return nil, fmt.Errorf("create_booking: user_id %q does not match this conversation's own lead", userID)
		}
		booking, err := w.glofoxClient.CreateBooking(ctx, glofox.CreateBookingInput{UserID: userID, EventID: eventID})
		if err != nil {
			// Explicit nil, not the (*glofox.CreateBookingResponse)(nil)
			// CreateBooking returns alongside its error — a typed nil
			// pointer wrapped in this function's `any` return is NOT
			// itself nil to a caller doing `result != nil`, which would
			// silently look like "got a result" even though the booking
			// actually failed. This is exactly the false-confirmation bug
			// task 7.5 exists to prevent — caught by
			// TestExecuteOneToolCall_CreateBooking_GlofoxFails_NeverFalseConfirms.
			return nil, err
		}
		return booking, nil

	default:
		return nil, fmt.Errorf("unknown tool %q", call.Tool)
	}
}

// appendToolResultsToPrompt builds the follow-up prompt for round 2
// (task 7.2) — the original prompt plus each executed tool call's
// outcome, so the model can incorporate real results instead of
// continuing to speculate.
func appendToolResultsToPrompt(originalPrompt string, calls []ToolCallRequest, results []ToolCallResult) string {
	var b []byte
	b = append(b, originalPrompt...)
	b = append(b, "\n\nTOOL CALL RESULTS (use these, don't re-request the same tools):\n"...)
	for i, r := range results {
		argsJSON, _ := json.Marshal(calls[i].Arguments)
		if r.Err != nil {
			b = append(b, []byte(fmt.Sprintf("- %s(%s) FAILED: %s\n", r.Tool, argsJSON, r.Err.Error()))...)
			continue
		}
		resultJSON, _ := json.Marshal(r.Result)
		b = append(b, []byte(fmt.Sprintf("- %s(%s) succeeded: %s\n", r.Tool, argsJSON, resultJSON))...)
	}
	return string(b)
}

func (w *AIWorker) auditToolCall(ctx context.Context, conversationID, studioID uuid.UUID, call ToolCallRequest, result any, callErr error) {
	argsJSON, _ := json.Marshal(call.Arguments)
	var resultJSON []byte
	if result != nil {
		resultJSON, _ = json.Marshal(result)
	}
	var errMsg *string
	if callErr != nil {
		s := callErr.Error()
		errMsg = &s
	}
	if err := w.msgRepo.RecordToolCallAudit(ctx, conversationID, studioID, call.Tool, argsJSON, resultJSON, errMsg); err != nil {
		w.log.Warn("tool call audit write failed", "tool", call.Tool, "conversation_id", conversationID, "err", err)
	}
}
