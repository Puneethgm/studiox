package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/leads"
)

func newTestWorkerWithGlofoxStub(t *testing.T, handler http.HandlerFunc) (*AIWorker, *httptest.Server) {
	t.Helper()
	stub := httptest.NewServer(handler)
	t.Cleanup(stub.Close)
	gf := glofox.NewWithBaseURL("key", "token", "branch1", stub.URL)
	return &AIWorker{glofoxClient: gf, msgRepo: &Repo{pool: (*pgxpool.Pool)(nil)}}, stub
}

// TestExecuteOneToolCall_CreateBooking_RejectsMismatchedLead covers task
// 7.3: a create_booking call for a user_id that doesn't match the
// conversation's own lead is rejected WITHOUT calling Glofox at all —
// the validation must happen before the HTTP call, not after.
func TestExecuteOneToolCall_CreateBooking_RejectsMismatchedLead(t *testing.T) {
	called := false
	w, _ := newTestWorkerWithGlofoxStub(t, func(rw http.ResponseWriter, r *http.Request) {
		called = true
		_ = json.NewEncoder(rw).Encode(glofox.CreateBookingResponse{Success: true})
	})
	lead := &leads.Lead{GlofoxMemberID: "real-member-123"}

	_, err := w.executeOneToolCall(context.Background(), uuid.Nil, lead, ToolCallRequest{
		Tool:      "create_booking",
		Arguments: map[string]any{"user_id": "attacker-supplied-456", "event_id": "evt1"},
	})
	if err == nil {
		t.Fatal("expected an error when user_id doesn't match the conversation's lead")
	}
	if called {
		t.Error("Glofox should never be called for a rejected tool call")
	}
}

// TestExecuteOneToolCall_CreateBooking_NoLead_Rejected covers the
// no-lead-linked-to-this-conversation case — must also reject, not
// silently allow because there's nothing to compare against.
func TestExecuteOneToolCall_CreateBooking_NoLead_Rejected(t *testing.T) {
	w, _ := newTestWorkerWithGlofoxStub(t, func(rw http.ResponseWriter, r *http.Request) {
		t.Error("Glofox should never be called when there's no lead to validate against")
	})

	_, err := w.executeOneToolCall(context.Background(), uuid.Nil, nil, ToolCallRequest{
		Tool:      "create_booking",
		Arguments: map[string]any{"user_id": "some-user", "event_id": "evt1"},
	})
	if err == nil {
		t.Fatal("expected an error when the conversation has no linked lead")
	}
}

// TestExecuteOneToolCall_CreateBooking_MatchingLead_Succeeds covers task
// 7.5: a correctly-scoped booking call succeeds and returns a real result
// only when the underlying Glofox call actually succeeds.
func TestExecuteOneToolCall_CreateBooking_MatchingLead_Succeeds(t *testing.T) {
	w, _ := newTestWorkerWithGlofoxStub(t, func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(rw).Encode(glofox.CreateBookingResponse{Success: true, BookingID: "bk1"})
	})
	lead := &leads.Lead{GlofoxMemberID: "member-123"}

	result, err := w.executeOneToolCall(context.Background(), uuid.Nil, lead, ToolCallRequest{
		Tool:      "create_booking",
		Arguments: map[string]any{"user_id": "member-123", "event_id": "evt1"},
	})
	if err != nil {
		t.Fatalf("executeOneToolCall: %v", err)
	}
	booking, ok := result.(*glofox.CreateBookingResponse)
	if !ok || booking.BookingID != "bk1" {
		t.Errorf("result = %+v, want a successful CreateBookingResponse with BookingID=bk1", result)
	}
}

// TestExecuteOneToolCall_CreateBooking_GlofoxFails_NeverFalseConfirms
// covers task 7.5's "a failed tool call never produces a false 'booked'
// confirmation" — the caller (ai_worker.go, via the follow-up prompt) can
// only know the booking failed if executeOneToolCall itself returns an
// error, which this confirms it does.
func TestExecuteOneToolCall_CreateBooking_GlofoxFails_NeverFalseConfirms(t *testing.T) {
	w, _ := newTestWorkerWithGlofoxStub(t, func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(rw).Encode(glofox.CreateBookingResponse{Success: false, Message: "class full"})
	})
	lead := &leads.Lead{GlofoxMemberID: "member-123"}

	result, err := w.executeOneToolCall(context.Background(), uuid.Nil, lead, ToolCallRequest{
		Tool:      "create_booking",
		Arguments: map[string]any{"user_id": "member-123", "event_id": "evt1"},
	})
	if err == nil {
		t.Fatal("expected an error when Glofox reports the booking failed")
	}
	if result != nil {
		t.Error("expected a nil result alongside the error")
	}
}

func TestExecuteOneToolCall_CheckAvailability_Succeeds(t *testing.T) {
	w, _ := newTestWorkerWithGlofoxStub(t, func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(rw).Encode(struct {
			Data []glofox.EventSlot `json:"data"`
		}{Data: []glofox.EventSlot{{EventID: "evt1", SpotsAvailable: 3}}})
	})

	result, err := w.executeOneToolCall(context.Background(), uuid.Nil, nil, ToolCallRequest{
		Tool:      "check_availability",
		Arguments: map[string]any{"from": float64(0), "to": float64(1000)},
	})
	if err != nil {
		t.Fatalf("executeOneToolCall: %v", err)
	}
	slots, ok := result.([]glofox.EventSlot)
	if !ok || len(slots) != 1 || slots[0].EventID != "evt1" {
		t.Errorf("result = %+v, want one evt1 slot", result)
	}
}

func TestExecuteOneToolCall_UnknownTool_Rejected(t *testing.T) {
	w, _ := newTestWorkerWithGlofoxStub(t, func(rw http.ResponseWriter, r *http.Request) {
		t.Error("Glofox should never be called for an unknown tool")
	})
	if _, err := w.executeOneToolCall(context.Background(), uuid.Nil, nil, ToolCallRequest{Tool: "delete_studio"}); err == nil {
		t.Fatal("expected an error for an unrecognized tool name")
	}
}

func TestExecuteOneToolCall_NoGlofoxClient_FailsClosed(t *testing.T) {
	w := &AIWorker{}
	if _, err := w.executeOneToolCall(context.Background(), uuid.Nil, nil, ToolCallRequest{Tool: "check_availability", Arguments: map[string]any{"from": float64(0), "to": float64(1)}}); err == nil {
		t.Fatal("expected an error when Glofox isn't configured, not a panic or silent success")
	}
}

func TestAppendToolResultsToPrompt_IncludesSuccessAndFailure(t *testing.T) {
	calls := []ToolCallRequest{
		{Tool: "check_availability", Arguments: map[string]any{"from": float64(0)}},
		{Tool: "create_booking", Arguments: map[string]any{"user_id": "u1"}},
	}
	results := []ToolCallResult{
		{Tool: "check_availability", Result: []glofox.EventSlot{{EventID: "evt1"}}},
		{Tool: "create_booking", Err: context.DeadlineExceeded},
	}
	got := appendToolResultsToPrompt("ORIGINAL PROMPT", calls, results)
	if !contains(got, "ORIGINAL PROMPT") {
		t.Error("follow-up prompt should still contain the original prompt")
	}
	if !contains(got, "evt1") {
		t.Error("follow-up prompt should include the successful tool's result")
	}
	if !contains(got, "FAILED") {
		t.Error("follow-up prompt should flag the failed tool call as failed")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
