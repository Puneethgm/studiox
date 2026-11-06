package messaging

import (
	"strings"
	"time"
)

// staffActiveWindowDefault is how recently a staff reply counts as "staff
// is actively handling this conversation right now" — the staff-active
// gate (task 6.1). Now a per-studio setting (studios.staff_active_window_minutes,
// see GetStaffActiveWindowMinutes); this is only the fallback used if that
// lookup fails.
const staffActiveWindowDefault = 10 * time.Minute

// staffActiveWithinWindow reports whether any staff (studio_user) message
// appears in history within window before now.
func staffActiveWithinWindow(history []Message, now time.Time, window time.Duration) bool {
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		if now.Sub(m.SentAt) > window {
			// history is chronological; once we're past the window, every
			// earlier message is too.
			break
		}
		if m.SourceKind == SourceStudioUser {
			return true
		}
	}
	return false
}

// dailyReplyCap is the per-conversation daily AI-reply cap (task 6.4) —
// doc-specified, not configurable.
const dailyReplyCap = 30

// aiStreakCap is the max consecutive AI replies allowed without an
// intervening staff message (task 6.5) — doc-specified, not configurable.
const aiStreakCap = 3

// aiStreakExceeded reports whether the trailing aiStreakCap outbound
// messages in history are ALL AI-sourced (no staff message among them) —
// meaning a reply right now would be the (aiStreakCap+1)th in a row.
func aiStreakExceeded(history []Message) bool {
	streak := 0
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		if m.Direction != DirectionOutbound {
			continue
		}
		if m.SourceKind != SourceAI {
			// Any non-AI outbound message (staff/automation) breaks the
			// streak.
			break
		}
		streak++
		if streak >= aiStreakCap {
			return true
		}
	}
	return false
}

// loopDetected reports whether the customer appears to be repeating the
// same message (task 6.6) — the last 2 inbound (customer) messages in
// history are near-identical to a 3rd one, i.e. the same question asked
// 3 times running without the conversation moving on. A simple
// normalized-text-equality heuristic, not semantic similarity — doc §4.2
// doesn't specify an exact algorithm, and an exact-repeat is already a
// strong, low-false-positive signal.
func loopDetected(history []Message) bool {
	var recent []string
	for i := len(history) - 1; i >= 0 && len(recent) < 3; i-- {
		m := history[i]
		if m.Direction != DirectionInbound {
			continue
		}
		recent = append(recent, normalizeForLoopCheck(m.Body))
	}
	if len(recent) < 3 {
		return false
	}
	return recent[0] == recent[1] && recent[1] == recent[2] && recent[0] != ""
}

func normalizeForLoopCheck(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// businessHoursAllows reports whether now (interpreted in loc) falls
// within any of slots' configured day/time windows. No slots configured
// at all means "no hours restriction" (always true) — a studio that
// hasn't set up availability slots shouldn't have its AI chat silently
// gated by an empty config. Times are "HH:MM-HH:MM" strings, the same
// format studios.AvailabilitySlot already uses for booking availability
// (reused here rather than inventing a second, parallel hours config —
// see design.md's Open Questions on this exact choice).
func businessHoursAllows(now time.Time, loc *time.Location, slots []AvailabilitySlotLike) bool {
	if len(slots) == 0 {
		return true
	}
	local := now.In(loc)
	weekday := strings.ToLower(local.Weekday().String())
	for _, s := range slots {
		if strings.ToLower(s.Day) != weekday {
			continue
		}
		for _, window := range s.Times {
			if timeWithinWindow(local, window) {
				return true
			}
		}
	}
	return false
}

// AvailabilitySlotLike mirrors studios.AvailabilitySlot's shape without
// importing internal/studios into this pure-logic file — keeps gates.go
// testable with plain fixtures. The real call site adapts
// studios.Studio.AvailabilitySlots into this.
type AvailabilitySlotLike struct {
	Day   string
	Times []string
}

// aiActiveHoursAllows reports whether now (interpreted in the studio's
// configured timezone) falls within the studio's configured "AI Active
// Hours" daily window — a separate, dedicated setting from the
// AvailabilitySlots-driven business-hours gate above. start/end are
// "HH:MM" strings. Unlike businessHoursAllows (which only ever checks a
// single same-day start<end window via timeWithinWindow), this supports
// an overnight window where end is earlier than start (e.g. "22:00" to
// "02:00") by treating "now is active" as "now >= start OR now < end" in
// that case, rather than always requiring now to fall between the two in
// calendar order.
func aiActiveHoursAllows(now time.Time, loc *time.Location, startHHMM, endHHMM string) bool {
	local := now.In(loc)
	start, err1 := time.ParseInLocation("15:04", strings.TrimSpace(startHHMM), local.Location())
	end, err2 := time.ParseInLocation("15:04", strings.TrimSpace(endHHMM), local.Location())
	if err1 != nil || err2 != nil {
		// Misconfigured window — fail open rather than silently blocking
		// every reply for this studio.
		return true
	}
	minsNow := local.Hour()*60 + local.Minute()
	startMins := start.Hour()*60 + start.Minute()
	endMins := end.Hour()*60 + end.Minute()
	if startMins == endMins {
		// A zero-width or full-day window ("00:00"-"00:00" etc.) — treat
		// as always-on rather than always-off, matching this migration's
		// own default values.
		return true
	}
	if startMins < endMins {
		return minsNow >= startMins && minsNow < endMins
	}
	// Overnight window (crosses midnight): active from start through
	// end-of-day, and again from start-of-day through end.
	return minsNow >= startMins || minsNow < endMins
}

func timeWithinWindow(t time.Time, window string) bool {
	parts := strings.SplitN(window, "-", 2)
	if len(parts) != 2 {
		return false
	}
	start, err1 := time.ParseInLocation("15:04", strings.TrimSpace(parts[0]), t.Location())
	end, err2 := time.ParseInLocation("15:04", strings.TrimSpace(parts[1]), t.Location())
	if err1 != nil || err2 != nil {
		return false
	}
	minsSinceMidnight := t.Hour()*60 + t.Minute()
	startMins := start.Hour()*60 + start.Minute()
	endMins := end.Hour()*60 + end.Minute()
	return minsSinceMidnight >= startMins && minsSinceMidnight < endMins
}
