package glofox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckAvailability_Success(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("x-glofox-branch-id") != "branch1" {
			t.Errorf("missing/wrong branch header: %q", r.Header.Get("x-glofox-branch-id"))
		}
		_ = json.NewEncoder(w).Encode(listEventsResponse{
			Data: []EventSlot{
				{EventID: "evt1", Name: "6pm HIIT", StartTimeUnix: 1000, SpotsAvailable: 5, Capacity: 20},
			},
		})
	}))
	defer stub.Close()

	c := NewWithBaseURL("key", "token", "branch1", stub.URL)
	slots, err := c.CheckAvailability(context.Background(), 0, 2000)
	if err != nil {
		t.Fatalf("CheckAvailability: %v", err)
	}
	if len(slots) != 1 || slots[0].EventID != "evt1" || slots[0].SpotsAvailable != 5 {
		t.Errorf("slots = %+v, want one evt1 slot with 5 spots", slots)
	}
}

func TestCheckAvailability_ErrorStatus(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer stub.Close()

	c := NewWithBaseURL("key", "token", "branch1", stub.URL)
	_, err := c.CheckAvailability(context.Background(), 0, 2000)
	if err == nil {
		t.Fatal("expected an error on a 500 response")
	}
}

func TestCheckAvailability_NilClient(t *testing.T) {
	var c *Client
	if _, err := c.CheckAvailability(context.Background(), 0, 100); err == nil {
		t.Error("expected an error from a nil (unconfigured) client, not a panic or silent success")
	}
}

func TestCreateBooking_Success(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["user_id"] != "user1" || body["event_id"] != "evt1" {
			t.Errorf("request body = %+v, want user_id=user1 event_id=evt1", body)
		}
		_ = json.NewEncoder(w).Encode(CreateBookingResponse{Success: true, BookingID: "bk1"})
	}))
	defer stub.Close()

	c := NewWithBaseURL("key", "token", "branch1", stub.URL)
	out, err := c.CreateBooking(context.Background(), CreateBookingInput{UserID: "user1", EventID: "evt1"})
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	if out.BookingID != "bk1" {
		t.Errorf("BookingID = %q, want %q", out.BookingID, "bk1")
	}
}

// TestCreateBooking_GlofoxReportsFailure_NeverReturnsAFalseSuccess is the
// critical contract test: design.md's reply-tools decision requires the
// caller to never surface a false "booked" confirmation — a success:false
// response from Glofox must come back as a Go error, not a nil error with
// a zero-value/empty result that a careless caller might treat as success.
func TestCreateBooking_GlofoxReportsFailure_NeverReturnsAFalseSuccess(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(CreateBookingResponse{Success: false, Message: "class is full"})
	}))
	defer stub.Close()

	c := NewWithBaseURL("key", "token", "branch1", stub.URL)
	out, err := c.CreateBooking(context.Background(), CreateBookingInput{UserID: "user1", EventID: "evt1"})
	if err == nil {
		t.Fatal("expected an error when Glofox reports success:false")
	}
	if out != nil {
		t.Error("expected a nil result alongside the error")
	}
}

func TestCreateBooking_MissingRequiredFields(t *testing.T) {
	c := NewWithBaseURL("key", "token", "branch1", "http://unused.invalid")
	if _, err := c.CreateBooking(context.Background(), CreateBookingInput{}); err == nil {
		t.Error("expected an error when UserID/EventID are empty")
	}
}

func TestCreateBooking_NilClient(t *testing.T) {
	var c *Client
	if _, err := c.CreateBooking(context.Background(), CreateBookingInput{UserID: "u", EventID: "e"}); err == nil {
		t.Error("expected an error from a nil (unconfigured) client, not a panic")
	}
}
