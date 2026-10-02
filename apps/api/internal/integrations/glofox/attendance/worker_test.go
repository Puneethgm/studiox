package attendance

import (
	"testing"
	"time"

	"github.com/projectx/api/internal/integrations/glofox"
)

func booking(attended bool, timeStart string) glofox.Booking {
	return glofox.Booking{UserID: "u1", Attended: attended, TimeStart: timeStart}
}

func TestCountInWindow_NoWindow_CountsLifetime(t *testing.T) {
	bookings := []glofox.Booking{
		booking(true, "2026-01-01 00:00:00"),
		booking(true, "2026-06-01 00:00:00"),
		booking(false, "2026-07-01 00:00:00"),
	}
	got := countInWindow(bookings, time.Time{}, time.Time{})
	if got != 2 {
		t.Errorf("countInWindow() = %d, want 2", got)
	}
}

func TestCountInWindow_ScopedToPlanPeriod(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	bookings := []glofox.Booking{
		booking(true, "2026-05-30 00:00:00"), // before window — excluded
		booking(true, "2026-06-01 00:00:00"), // on start — included
		booking(true, "2026-06-10 00:00:00"), // inside — included
		booking(true, "2026-06-15 00:00:00"), // on end — included
		booking(true, "2026-06-20 00:00:00"), // after window — excluded
		booking(false, "2026-06-10 00:00:00"),
	}
	got := countInWindow(bookings, start, end)
	if got != 3 {
		t.Errorf("countInWindow() = %d, want 3", got)
	}
}

func TestCountInWindow_UnparseableDate_IncludedRegardless(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	got := countInWindow([]glofox.Booking{booking(true, "not-a-date")}, start, end)
	if got != 1 {
		t.Errorf("countInWindow() = %d, want 1 (unparseable dates are included, not dropped)", got)
	}
}

func TestBooking_ParsedTimeStart(t *testing.T) {
	b := glofox.Booking{TimeStart: "2026-08-02 00:00:00"}
	got, ok := b.ParsedTimeStart()
	if !ok {
		t.Fatal("ParsedTimeStart() ok = false, want true")
	}
	want := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParsedTimeStart() = %v, want %v", got, want)
	}

	if _, ok := (glofox.Booking{TimeStart: ""}).ParsedTimeStart(); ok {
		t.Error("ParsedTimeStart() on empty string: ok = true, want false")
	}
}
