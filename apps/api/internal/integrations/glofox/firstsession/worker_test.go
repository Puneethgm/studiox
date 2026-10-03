package firstsession

import (
	"testing"
	"time"

	"github.com/projectx/api/internal/integrations/glofox"
)

func TestAttendedSince(t *testing.T) {
	since := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	bookings := []glofox.Booking{
		{UserID: "old-attendee", Attended: true, TimeStart: "2026-09-21 10:00:00"},    // before the switch was turned on
		{UserID: "recent-attendee", Attended: true, TimeStart: "2026-10-03 09:00:00"}, // after
		{UserID: "recent-no-show", Attended: false, TimeStart: "2026-10-03 09:00:00"}, // not attended
		{UserID: "no-time", Attended: true, TimeStart: ""},                            // unknown time must not count
		{UserID: "bad-time", Attended: true, TimeStart: "03/10/2026"},                 // unparseable must not count
		{UserID: "recent-attendee", Attended: true, TimeStart: "2026-09-01 10:00:00"}, // duplicate user, extra old booking
		{UserID: "boundary", Attended: true, TimeStart: "2026-10-03 00:00:00"},        // exactly at since counts
	}

	got := attendedSince(bookings, since)

	for _, id := range []string{"recent-attendee", "boundary"} {
		if !got[id] {
			t.Errorf("expected %q to be eligible", id)
		}
	}
	for _, id := range []string{"old-attendee", "recent-no-show", "no-time", "bad-time"} {
		if got[id] {
			t.Errorf("expected %q to be excluded", id)
		}
	}
	if len(got) != 2 {
		t.Errorf("got %d eligible users %v, want 2", len(got), got)
	}
}
