package messaging

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/studios"
)

// checkReplyGates runs every reply gate (ai-reply-flow-llm-design task 6)
// and returns the first one that blocks this reply (a short machine-
// readable reason, e.g. "staff_active") or "" if none do. Gates run
// cheapest/most-likely-to-fire first.
func (w *AIWorker) checkReplyGates(ctx context.Context, studioID uuid.UUID, conv *Conversation, studio *studios.Studio, history []Message) string {
	now := time.Now()

	// 6.1 Staff-active: a human is already handling this. Window is
	// per-studio (default 10min, see migration 20261104020001) — fall back
	// to the old hardcoded default only if the lookup itself fails, so a
	// transient DB error degrades to the safe prior behavior rather than
	// disabling the gate outright.
	window := staffActiveWindowDefault
	if studioID != uuid.Nil {
		if minutes, err := w.studiosRepo.GetStaffActiveWindowMinutes(ctx, studioID); err == nil {
			window = time.Duration(minutes) * time.Minute
		}
	}
	if staffActiveWithinWindow(history, now, window) {
		return "staff_active"
	}

	// 6.5 AI-message-streak: don't send a 4th AI reply in a row unprompted.
	if aiStreakExceeded(history) {
		return "ai_streak_exceeded"
	}

	// 6.6 Loop detection: the same question asked 3 times running.
	if loopDetected(history) {
		return "loop_detected"
	}

	// 6.2 Business hours — only enforced if the studio has opted in
	// (ai_after_hours_behavior='skip'); default 'allow' never gates.
	if studio != nil {
		behavior, err := w.msgRepo.aiAfterHoursBehavior(ctx, studioID)
		if err == nil && behavior == "skip" {
			loc := time.UTC
			if studio.AvailabilityTimezone != "" {
				if l, lerr := time.LoadLocation(studio.AvailabilityTimezone); lerr == nil {
					loc = l
				}
			}
			slots := make([]AvailabilitySlotLike, 0, len(studio.AvailabilitySlots))
			for _, s := range studio.AvailabilitySlots {
				slots = append(slots, AvailabilitySlotLike{Day: s.Day, Times: s.Times})
			}
			if !businessHoursAllows(now, loc, slots) {
				return "outside_business_hours"
			}
		}
	}

	// AI Active Hours — a separate, dedicated daily on/off window (distinct
	// from the AvailabilitySlots-driven business-hours gate above), only
	// enforced if the studio has turned it on (disabled by default).
	if cfg, err := w.msgRepo.GetAIActiveHours(ctx, studioID); err == nil && cfg.Enabled {
		loc := time.UTC
		if cfg.Timezone != "" {
			if l, lerr := time.LoadLocation(cfg.Timezone); lerr == nil {
				loc = l
			}
		}
		if !aiActiveHoursAllows(now, loc, cfg.Start, cfg.End) {
			return "outside_ai_active_hours"
		}
	}

	// 6.3 Per-studio daily budget — only enforced if a budget is actually
	// configured (NULL means unlimited, the default).
	if budget, err := w.msgRepo.aiDailyBudgetCents(ctx, studioID); err == nil && budget != nil {
		if spent, err := w.msgRepo.TodayAISpendUSDCents(ctx, studioID); err == nil && spent >= *budget {
			return "daily_budget_exceeded"
		}
	}

	// 6.4 Per-conversation daily reply cap.
	if count, err := w.msgRepo.TodayAIReplyCountForConversation(ctx, conv.ID); err == nil && count >= dailyReplyCap {
		return "daily_reply_cap_exceeded"
	}

	return ""
}
