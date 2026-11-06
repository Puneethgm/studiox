package messaging

import (
	"testing"
	"time"
)

func msgAt(sourceKind SourceKind, direction Direction, body string, t time.Time) Message {
	return Message{SourceKind: sourceKind, Direction: direction, Body: body, SentAt: t}
}

func TestStaffActiveWithinWindow(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		history []Message
		want    bool
	}{
		{
			name: "recent staff message within window",
			history: []Message{
				msgAt(SourceCustomer, DirectionInbound, "hi", now.Add(-5*time.Minute)),
				msgAt(SourceStudioUser, DirectionOutbound, "hey there", now.Add(-2*time.Minute)),
			},
			want: true,
		},
		{
			name: "staff message outside window",
			history: []Message{
				msgAt(SourceStudioUser, DirectionOutbound, "hey there", now.Add(-20*time.Minute)),
			},
			want: false,
		},
		{
			name: "no staff message at all",
			history: []Message{
				msgAt(SourceCustomer, DirectionInbound, "hi", now.Add(-1*time.Minute)),
				msgAt(SourceAI, DirectionOutbound, "hello!", now.Add(-30*time.Second)),
			},
			want: false,
		},
		{
			name:    "empty history",
			history: nil,
			want:    false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := staffActiveWithinWindow(c.history, now, staffActiveWindow); got != c.want {
				t.Errorf("staffActiveWithinWindow() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAIStreakExceeded(t *testing.T) {
	now := time.Now()
	mk := func(n int, kind SourceKind) []Message {
		var out []Message
		for i := 0; i < n; i++ {
			out = append(out, msgAt(kind, DirectionOutbound, "reply", now.Add(time.Duration(i)*time.Minute)))
		}
		return out
	}

	cases := []struct {
		name    string
		history []Message
		want    bool
	}{
		{"3 AI replies in a row exceeds cap", mk(3, SourceAI), true},
		{"2 AI replies does not exceed cap", mk(2, SourceAI), false},
		{"4 AI replies in a row exceeds cap", mk(4, SourceAI), true},
		{"staff message breaks the streak", append(mk(1, SourceStudioUser), mk(2, SourceAI)...), false},
		{"empty history", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := aiStreakExceeded(c.history); got != c.want {
				t.Errorf("aiStreakExceeded() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAIStreakExceeded_IgnoresInboundMessages(t *testing.T) {
	now := time.Now()
	history := []Message{
		msgAt(SourceAI, DirectionOutbound, "reply 1", now),
		msgAt(SourceCustomer, DirectionInbound, "ok", now.Add(time.Minute)),
		msgAt(SourceAI, DirectionOutbound, "reply 2", now.Add(2*time.Minute)),
		msgAt(SourceCustomer, DirectionInbound, "ok", now.Add(3*time.Minute)),
		msgAt(SourceAI, DirectionOutbound, "reply 3", now.Add(4*time.Minute)),
	}
	if !aiStreakExceeded(history) {
		t.Error("3 AI outbound replies with customer messages interleaved should still count as a streak of 3")
	}
}

func TestLoopDetected(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		history []Message
		want    bool
	}{
		{
			name: "same question 3 times running",
			history: []Message{
				msgAt(SourceCustomer, DirectionInbound, "how much is it?", now),
				msgAt(SourceAI, DirectionOutbound, "it's $99", now.Add(time.Minute)),
				msgAt(SourceCustomer, DirectionInbound, "How much is it?", now.Add(2*time.Minute)),
				msgAt(SourceAI, DirectionOutbound, "it's $99", now.Add(3*time.Minute)),
				msgAt(SourceCustomer, DirectionInbound, "  how much is it?  ", now.Add(4*time.Minute)),
			},
			want: true,
		},
		{
			name: "different questions, not a loop",
			history: []Message{
				msgAt(SourceCustomer, DirectionInbound, "how much is it?", now),
				msgAt(SourceCustomer, DirectionInbound, "what about hours?", now.Add(time.Minute)),
				msgAt(SourceCustomer, DirectionInbound, "any offers?", now.Add(2*time.Minute)),
			},
			want: false,
		},
		{
			name: "fewer than 3 customer messages",
			history: []Message{
				msgAt(SourceCustomer, DirectionInbound, "hi", now),
				msgAt(SourceCustomer, DirectionInbound, "hi", now.Add(time.Minute)),
			},
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := loopDetected(c.history); got != c.want {
				t.Errorf("loopDetected() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTimeWithinWindow(t *testing.T) {
	loc := time.UTC
	mk := func(hh, mm int) time.Time {
		return time.Date(2026, 1, 5, hh, mm, 0, 0, loc) // a Monday
	}

	cases := []struct {
		name   string
		t      time.Time
		window string
		want   bool
	}{
		{"within window", mk(10, 0), "09:00-17:00", true},
		{"before window", mk(8, 0), "09:00-17:00", false},
		{"after window", mk(18, 0), "09:00-17:00", false},
		{"exactly at start", mk(9, 0), "09:00-17:00", true},
		{"exactly at end is exclusive", mk(17, 0), "09:00-17:00", false},
		{"malformed window", mk(10, 0), "not-a-window", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := timeWithinWindow(c.t, c.window); got != c.want {
				t.Errorf("timeWithinWindow(%v, %q) = %v, want %v", c.t, c.window, got, c.want)
			}
		})
	}
}

func TestBusinessHoursAllows(t *testing.T) {
	loc := time.UTC
	monday10am := time.Date(2026, 1, 5, 10, 0, 0, 0, loc) // Monday
	sunday10am := time.Date(2026, 1, 4, 10, 0, 0, 0, loc) // Sunday

	slots := []AvailabilitySlotLike{
		{Day: "monday", Times: []string{"09:00-17:00"}},
	}

	if !businessHoursAllows(monday10am, loc, slots) {
		t.Error("Monday 10am should be within the configured Monday 9-5 window")
	}
	if businessHoursAllows(sunday10am, loc, slots) {
		t.Error("Sunday should not be allowed when only Monday is configured")
	}
	if !businessHoursAllows(sunday10am, loc, nil) {
		t.Error("no slots configured at all should mean no hours restriction")
	}
}

func TestAIActiveHoursAllows(t *testing.T) {
	loc := time.UTC
	mk := func(hh, mm int) time.Time {
		return time.Date(2026, 1, 5, hh, mm, 0, 0, loc)
	}

	cases := []struct {
		name  string
		t     time.Time
		start string
		end   string
		want  bool
	}{
		{"inside same-day window", mk(10, 0), "09:00", "17:00", true},
		{"before same-day window", mk(8, 0), "09:00", "17:00", false},
		{"after same-day window", mk(18, 0), "09:00", "17:00", false},
		{"exactly at start is inclusive", mk(9, 0), "09:00", "17:00", true},
		{"exactly at end is exclusive", mk(17, 0), "09:00", "17:00", false},
		// Overnight window crossing midnight (22:00-02:00): active from
		// 22:00 through 23:59, and again from 00:00 through 01:59.
		{"overnight window, late evening", mk(23, 0), "22:00", "02:00", true},
		{"overnight window, early morning", mk(1, 0), "22:00", "02:00", true},
		{"overnight window, exactly at start", mk(22, 0), "22:00", "02:00", true},
		{"overnight window, exactly at end is exclusive", mk(2, 0), "22:00", "02:00", false},
		{"overnight window, daytime is outside", mk(12, 0), "22:00", "02:00", false},
		{"malformed start fails open", mk(10, 0), "not-a-time", "17:00", true},
		{"malformed end fails open", mk(10, 0), "09:00", "not-a-time", true},
		{"zero-width window treated as always-on", mk(3, 0), "00:00", "00:00", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := aiActiveHoursAllows(c.t, loc, c.start, c.end); got != c.want {
				t.Errorf("aiActiveHoursAllows(%v, %q-%q) = %v, want %v", c.t, c.start, c.end, got, c.want)
			}
		})
	}
}
