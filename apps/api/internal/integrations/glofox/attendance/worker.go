package attendance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/crm"
	"github.com/projectx/api/internal/integrations/glofox"
)

const pollInterval = 15 * time.Minute

// Worker polls Glofox every 15 minutes, once per studio that has connected
// its own Glofox account via the CRM connector (crm_connections — the same
// one studios use to connect Mindbody etc.), and stores a per-studio
// attendance snapshot via Repo for the Attendance page to read.
//
// Deliberately NOT backed by the platform-wide GLOFOX_API_KEY/API_TOKEN/
// BRANCH_ID env vars — those still back the older firstsession worker and
// lead-sync, which remain single-studio/env-configured for now. This worker
// reads exclusively from each studio's own saved CRM connection, so a
// studio only shows up here once *that studio's admin* has connected
// Glofox, and only ever sees its own data.
//
// It never sends a message — this is a read-only data feed.
type Worker struct {
	crmRepo *crm.Repo
	repo    *Repo
	log     *slog.Logger
}

func New(crmRepo *crm.Repo, repo *Repo, log *slog.Logger) *Worker {
	return &Worker{crmRepo: crmRepo, repo: repo, log: log}
}

func (w *Worker) Run(ctx context.Context) {
	w.log.Info("Glofox | Attendance worker started", "component", "glofox_attendance", "poll_interval", pollInterval)

	// Fire immediately on startup so the page isn't empty until the first tick.
	w.tick(ctx)

	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("Glofox | Attendance worker stopping", "component", "glofox_attendance")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// tick finds every studio with its own active Glofox CRM connection and
// polls each one in turn with that studio's own credentials. Sequential,
// not concurrent — Glofox has no documented rate limit, but there's no
// reason to hit it from N goroutines at once for what's a 15-minute cadence.
func (w *Worker) tick(ctx context.Context) {
	provider, err := w.crmRepo.GetProviderByName(ctx, "Glofox")
	if err != nil {
		w.log.Warn("Glofox | Attendance worker — Glofox provider not found in crm_providers, skipping cycle",
			"component", "glofox_attendance", "error", err.Error())
		return
	}

	connections, err := w.crmRepo.ListActiveConnectionsByProvider(ctx, provider.ID)
	if err != nil {
		w.log.Warn("Glofox | Attendance worker — failed to list studio connections, will retry next poll",
			"component", "glofox_attendance", "error", err.Error())
		return
	}
	if len(connections) == 0 {
		w.log.Debug("Glofox | Attendance worker — no studio has connected Glofox yet", "component", "glofox_attendance")
		return
	}

	for _, conn := range connections {
		w.tickStudio(ctx, conn)
	}
}

// TickStudio polls Glofox for a single studio right away, instead of
// waiting for the next scheduled 15-minute cycle — called when a studio
// admin connects Glofox for the first time, so the Attendance page doesn't
// sit empty until the shared ticker gets around to it. A no-op (not an
// error) if the studio's active CRM connection isn't actually Glofox, so
// callers can fire this unconditionally on every CRM connect without
// checking the provider themselves first.
func (w *Worker) TickStudio(ctx context.Context, studioID uuid.UUID) error {
	provider, err := w.crmRepo.GetProviderByName(ctx, "Glofox")
	if err != nil {
		return fmt.Errorf("get glofox provider: %w", err)
	}
	conn, err := w.crmRepo.GetActiveConnection(ctx, studioID, provider.ID)
	if err != nil {
		if errors.Is(err, crm.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get studio's glofox connection: %w", err)
	}
	w.tickStudio(ctx, *conn)
	return nil
}

func (w *Worker) tickStudio(ctx context.Context, conn crm.Connection) {
	creds, err := w.crmRepo.DecryptCredentials(&conn)
	if err != nil {
		w.log.Warn("Glofox | Attendance worker — failed to decrypt studio's Glofox credentials",
			"component", "glofox_attendance", "studio_id", conn.StudioID, "error", err.Error())
		return
	}
	// Field keys match crm_providers.auth_field_defs for Glofox — the same
	// header names glofox.Client.addAuth sets on every request.
	gf := glofox.New(creds["x-api-key"], creds["x-glofox-api-token"], creds["x-glofox-branch-id"])
	if gf == nil {
		w.log.Warn("Glofox | Attendance worker — studio's Glofox connection is missing a required field",
			"component", "glofox_attendance", "studio_id", conn.StudioID)
		return
	}

	listCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	bookings, err := gf.ListBookings(listCtx)
	if err != nil {
		w.log.Warn("Glofox | Attendance worker — failed to fetch bookings, will retry next poll",
			"component", "glofox_attendance", "studio_id", conn.StudioID, "error", err.Error())
		return
	}

	threshold, err := w.repo.GetQualifyingThreshold(ctx, conn.StudioID)
	if err != nil {
		w.log.Warn("Glofox | Attendance worker — failed to load threshold, using default",
			"component", "glofox_attendance", "studio_id", conn.StudioID, "error", err.Error())
		threshold = defaultQualifyingThreshold
	}

	byUser := make(map[string][]glofox.Booking)
	for _, b := range bookings {
		if b.UserID == "" {
			continue
		}
		byUser[b.UserID] = append(byUser[b.UserID], b)
	}

	// Scoped to each member's current plan period (start_date..expiry_date)
	// rather than lifetime — GetMember is required for every member here
	// (not just qualifiers) since we can't know their plan window, or
	// whether they even qualify, without it. The real cost of this: one
	// Glofox call per member per poll instead of per qualifier.
	//
	// Each GetMember gets its OWN short timeout rather than sharing one
	// budget across the whole batch — a shared timeout sized for one call
	// is way too short for hundreds of them, but sized for hundreds of
	// calls it lets a few slow/stuck requests burn the entire remaining
	// budget and silently fail every member queued after them (this is
	// exactly what happened before this was per-call: ~27% of members in a
	// 616-member branch fell back to lifetime counts because GetMember
	// calls late in the loop timed out).
	// Each member's row is written as soon as it's computed (UpsertRow, one
	// statement, no shared transaction) rather than batched into one write
	// at the end — the Attendance page fills in progressively as the poll
	// runs instead of staying empty for however long hundreds of Glofox
	// calls take, and a timeout or crash partway through doesn't lose
	// members already processed.
	atOrAboveThreshold := 0
	for userID, userBookings := range byUser {
		row := Row{GlofoxUserID: userID, Name: userBookings[0].UserName}

		memberCtx, memberCancel := context.WithTimeout(ctx, 20*time.Second)
		member, err := gf.GetMember(memberCtx, userID)
		memberCancel()
		if err != nil {
			w.log.Warn("Glofox | Attendance worker — failed to resolve member, counting lifetime attendance as a fallback",
				"component", "glofox_attendance", "studio_id", conn.StudioID, "glofox_user_id", userID, "error", err.Error())
			row.ClassesAttended = countInWindow(userBookings, time.Time{}, time.Time{})
			if err := w.repo.UpsertRow(ctx, conn.StudioID, row); err != nil {
				w.log.Warn("Glofox | Attendance worker — failed to store row",
					"component", "glofox_attendance", "studio_id", conn.StudioID, "glofox_user_id", userID, "error", err.Error())
			}
			continue
		}

		if n := member.FirstName + " " + member.LastName; n != " " {
			row.Name = n
		}

		var windowStart, windowEnd time.Time
		if member.Membership.Status == "ACTIVE" && member.Membership.StartDate > 0 {
			windowStart = time.Unix(member.Membership.StartDate, 0)
			if member.Membership.ExpiryDate > 0 {
				windowEnd = time.Unix(member.Membership.ExpiryDate, 0)
			} else {
				windowEnd = time.Now() // open-ended plan — count up to now
			}
		}
		// No active plan on record (expired, PAYG-only, or Glofox has no
		// start_date) — fall back to lifetime, there's no window to scope to.
		row.ClassesAttended = countInWindow(userBookings, windowStart, windowEnd)
		row.PlanName = member.Membership.MembershipName
		if member.Membership.BookedEvents > 0 {
			row.PlanLimit = member.Membership.BookedEvents
		}
		if !windowStart.IsZero() {
			row.PlanStart = &windowStart
			if member.Membership.ExpiryDate > 0 {
				row.PlanEnd = &windowEnd
			}
		}

		if row.ClassesAttended >= threshold {
			row.Phone = member.Phone
			row.Email = member.ResolveEmail()
			atOrAboveThreshold++
		}

		if err := w.repo.UpsertRow(ctx, conn.StudioID, row); err != nil {
			w.log.Warn("Glofox | Attendance worker — failed to store row",
				"component", "glofox_attendance", "studio_id", conn.StudioID, "glofox_user_id", userID, "error", err.Error())
		}
	}

	w.log.Info("Glofox | Attendance worker cycle complete",
		"component", "glofox_attendance",
		"studio_id", conn.StudioID,
		"total_bookings", len(bookings),
		"distinct_members", len(byUser),
		"threshold", threshold,
		"members_at_or_above_threshold", atOrAboveThreshold,
	)
}

// countInWindow counts a member's attended bookings whose time_start falls
// within [start, end] inclusive. A zero start means "no lower bound"
// (lifetime); a booking whose time_start fails to parse is counted
// regardless of the window — safer to include an unparseable date than to
// silently undercount.
func countInWindow(bookings []glofox.Booking, start, end time.Time) int {
	count := 0
	for _, b := range bookings {
		if !b.Attended {
			continue
		}
		if start.IsZero() {
			count++
			continue
		}
		t, ok := b.ParsedTimeStart()
		if !ok {
			count++
			continue
		}
		if !t.Before(start) && !t.After(end) {
			count++
		}
	}
	return count
}
