package glofox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ai-reply-flow-llm-design task 7.1: CheckAvailability/CreateBooking,
// the live actions the reply-tools capability (doc §4.7) calls.
//
// IMPORTANT — endpoint shape not independently confirmed: Glofox's
// booking-write API sits behind a gated developer portal
// (gf-dev-docs.glofox.com / apidocs-plat.aws.glofox.com) that requires
// requesting access via apiactivation@abcfitness.com; this session has
// no credentials for it and a web search turned up no public
// documentation for the exact request/response shape of these two
// calls. The endpoints/payloads below follow this file's own
// established conventions (same base URL, same x-glofox-*/x-api-key
// auth, same `/2.x/branches/{branchId}/...` path shape, same
// success/message response envelope PurchaseMembershipResponse already
// uses) as the most-likely-correct guess given everything else this
// client does — but this must be confirmed (and the path/field names
// corrected if needed) against a real Glofox sandbox account before
// going live. Not marked done-without-caveat; see
// openspec/changes/ai-reply-flow-llm-design/tasks.md task 7.1's note.

// EventSlot is one bookable class/session, as returned by
// CheckAvailability.
type EventSlot struct {
	EventID        string `json:"_id"`
	Name           string `json:"name"`
	StartTimeUnix  int64  `json:"start_time"`
	EndTimeUnix    int64  `json:"end_time"`
	SpotsAvailable int    `json:"spots_available"`
	Capacity       int    `json:"capacity"`
}

type listEventsResponse struct {
	Data []EventSlot `json:"data"`
}

// CheckAvailability fetches bookable events for the branch within
// [fromUnix, toUnix) via GET /2.0/branches/{branchId}/events.
func (c *Client) CheckAvailability(ctx context.Context, fromUnix, toUnix int64) ([]EventSlot, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}

	url := fmt.Sprintf("%s/2.0/branches/%s/events?from=%d&to=%d", c.baseURL, c.branchID, fromUnix, toUnix)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out listEventsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}
	return out.Data, nil
}

// CreateBookingInput holds the fields sent to
// POST /2.0/branches/{branchId}/bookings.
type CreateBookingInput struct {
	UserID  string // Glofox user id
	EventID string // the event/class session to book, from CheckAvailability
}

// CreateBookingResponse is Glofox's reply to a booking call — same
// success/message envelope PurchaseMembershipResponse uses elsewhere in
// this client.
type CreateBookingResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	BookingID string `json:"booking_id"`
}

// CreateBooking books userID into eventID. Returns an error (never a
// silently-successful zero value) if Glofox reports anything other than
// success — the reply-tools caller (design.md's "application-level
// validation" decision) must never surface a false "booked" confirmation
// to a customer when this call actually failed.
func (c *Client) CreateBooking(ctx context.Context, in CreateBookingInput) (*CreateBookingResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}
	if in.UserID == "" || in.EventID == "" {
		return nil, fmt.Errorf("glofox: userID and eventID are required")
	}

	body := map[string]any{
		"user_id":  in.UserID,
		"event_id": in.EventID,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/2.0/branches/"+c.branchID+"/bookings", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out CreateBookingResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("glofox create booking failed: %s", out.Message)
	}
	return &out, nil
}
