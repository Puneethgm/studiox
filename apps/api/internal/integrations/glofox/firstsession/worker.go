// Package firstsession polls Glofox for members who have purchased a plan and
// attended at least one session, then sends them an automated WhatsApp message
// via the existing outbound job infrastructure.
//
// All contact data (name, phone) is pulled from Glofox so the worker functions
// even when the member has no matching StudioX lead. A StudioX lead is linked
// to the conversation if one is found by email — this enriches inbox context
// but is never a blocker.
package firstsession

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging"
)

const pollInterval = 30 * time.Minute

// Worker polls Glofox every 30 minutes and fires a WhatsApp first-session message
// for any member with an active plan who attended at least one session.
type Worker struct {
	gf        *glofox.Client
	pool      *pgxpool.Pool // for glofox_first_session_log table
	leadsRepo *leads.Repo
	msgSvc    *messaging.Service
	msgRepo   *messaging.Repo
	studioID  uuid.UUID
	branchID  string // Glofox branch ID — part of the log table primary key
	log       *slog.Logger
}

func New(
	gf *glofox.Client,
	pool *pgxpool.Pool,
	leadsRepo *leads.Repo,
	msgSvc *messaging.Service,
	msgRepo *messaging.Repo,
	studioID uuid.UUID,
	branchID string,
	log *slog.Logger,
) *Worker {
	return &Worker{
		gf:        gf,
		pool:      pool,
		leadsRepo: leadsRepo,
		msgSvc:    msgSvc,
		msgRepo:   msgRepo,
		studioID:  studioID,
		branchID:  branchID,
		log:       log,
	}
}

func (w *Worker) Run(ctx context.Context) {
	if w.gf == nil || w.studioID == uuid.Nil {
		w.log.Info("Glofox | First-session worker disabled — set GLOFOX_STUDIO_ID to enable",
			"component", "glofox_first_session")
		return
	}
	w.log.Info("Glofox | First-session worker started",
		"component", "glofox_first_session",
		"poll_interval", pollInterval,
		"studio_id", w.studioID,
	)

	// Fire immediately on startup so admins don't wait 30 min.
	w.tick(ctx)

	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("Glofox | First-session worker stopping", "component", "glofox_first_session")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	tCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	bookings, err := w.gf.ListBookings(tCtx)
	if err != nil {
		w.log.Warn("Glofox | First-session worker — failed to fetch bookings, will retry next poll",
			"component", "glofox_first_session", "error", err.Error())
		return
	}

	// Collect unique Glofox user IDs where at least one booking is attended.
	attendedUsers := map[string]bool{}
	for _, b := range bookings {
		if b.Attended {
			attendedUsers[b.UserID] = true
		}
	}

	if len(attendedUsers) == 0 {
		w.log.Debug("Glofox | First-session worker — no attended bookings in current batch",
			"component", "glofox_first_session", "total_bookings", len(bookings))
		return
	}

	newlyNotified := 0
	for userID := range attendedUsers {
		notified, err := w.processUser(tCtx, userID)
		if err != nil {
			w.log.Warn("Glofox | First-session worker — failed to process member",
				"component", "glofox_first_session", "glofox_user_id", userID, "error", err.Error())
		}
		if notified {
			newlyNotified++
		}
	}

	w.log.Info("Glofox | First-session worker cycle complete",
		"component", "glofox_first_session",
		"total_bookings", len(bookings),
		"attended_members", len(attendedUsers),
		"newly_notified", newlyNotified,
	)
}

func (w *Worker) processUser(ctx context.Context, glofoxUserID string) (bool, error) {
	// --- 1. Fetch full member profile from Glofox ---
	member, err := w.gf.GetMember(ctx, glofoxUserID)
	if err != nil {
		return false, fmt.Errorf("get member from Glofox: %w", err)
	}

	// --- 2. Only proceed when they have an active plan ---
	if !member.HasActivePlan() {
		return false, nil
	}

	// --- 3. Phone is required for WhatsApp; Glofox is the source of truth here ---
	phone := member.Phone
	if phone == "" {
		return false, nil // no phone in Glofox — cannot message
	}

	// --- 4. Atomic de-dup via log table: claim the slot or skip if already done ---
	inserted, err := w.claimNotificationSlot(ctx, glofoxUserID)
	if err != nil {
		return false, fmt.Errorf("claim notification slot: %w", err)
	}
	if !inserted {
		return false, nil // already notified in a previous poll
	}

	// --- 5. Build display name from Glofox data ---
	displayName := strings.TrimSpace(member.FirstName + " " + member.LastName)
	if displayName == "" {
		displayName = member.ResolveEmail()
	}

	// --- 6. Optional: find a matching StudioX lead to link the conversation ---
	var leadID *uuid.UUID
	email := member.ResolveEmail()
	if email != "" {
		if lead, err := w.leadsRepo.FindLeadByEmail(ctx, w.studioID, email); err == nil {
			leadID = &lead.ID
			// Also mark the leads-table column so the UI shows the right state.
			_, _ = w.leadsRepo.MarkGlofoxFirstSessionNotified(ctx, lead.ID)
		} else if !errors.Is(err, leads.ErrLeadNotFound) {
			w.log.Warn("Glofox | First-session worker — lead lookup error (continuing without link)",
				"component", "glofox_first_session", "email", email, "error", err.Error())
		}
	}

	// --- 7. Resolve best available channel ---
	channelKind := messaging.KindWhatsAppMeta
	if _, err := w.msgRepo.GetActiveChannelByKind(ctx, w.studioID, messaging.KindWhatsAppMeta); err != nil {
		if _, waWebErr := w.msgRepo.GetActiveChannelByKind(ctx, w.studioID, messaging.KindWhatsAppWeb); waWebErr == nil {
			channelKind = messaging.KindWhatsAppWeb
		} else if _, smsErr := w.msgRepo.GetActiveChannelByKind(ctx, w.studioID, messaging.KindSMS); smsErr == nil {
			channelKind = messaging.KindSMS
		} else {
			w.log.Warn("Glofox | First-session worker — no active WhatsApp or SMS channel for studio, skipping",
				"component", "glofox_first_session",
				"studio_id", w.studioID,
				"glofox_user_id", glofoxUserID,
			)
			return false, nil
		}
	}

	// --- 8. Create or find conversation ---
	conv, err := w.msgSvc.CreateConversation(ctx, w.studioID, messaging.CreateConversationInput{
		ChannelKind:  channelKind,
		ContactValue: phone,
		DisplayName:  displayName,
		LeadID:       leadID, // nil if no matching StudioX lead
	})
	if err != nil {
		return false, fmt.Errorf("create conversation: %w", err)
	}

	// --- 9. Enqueue first-session message ---
	// Uses {{contact.first_name}} which resolves from the Glofox display name above.
	body := "Hi {{contact.first_name}}, congrats on completing your first session at {{studio.name}}! We hope you loved it. Are you ready to commit to your fitness journey? Please select:\n1. Yes, let's do this!\n2. Tell me more about membership"

	if _, err := w.msgRepo.EnqueueOutbound(ctx, messaging.OutboundJob{
		StudioID:       w.studioID,
		ConversationID: conv.ID,
		Body:           body,
		SourceKind:     messaging.SourceAutomation,
		SourceRef:      fmt.Sprintf("glofox:%s:%s:first_session", w.branchID, glofoxUserID),
		ScheduledFor:   time.Now().UTC(),
	}); err != nil {
		return false, fmt.Errorf("enqueue outbound: %w", err)
	}

	w.log.Info("Glofox | First-session WhatsApp sent",
		"component", "glofox_first_session",
		"glofox_user_id", glofoxUserID,
		"display_name", displayName,
		"phone", phone,
		"channel", channelKind,
		"lead_linked", leadID != nil,
	)
	return true, nil
}

// claimNotificationSlot inserts a row into glofox_first_session_log.
// Returns true only on the first insert (i.e., not a duplicate).
func (w *Worker) claimNotificationSlot(ctx context.Context, glofoxUserID string) (bool, error) {
	tag, err := w.pool.Exec(ctx, `
		INSERT INTO glofox_first_session_log (glofox_user_id, branch_id, studio_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (glofox_user_id, branch_id) DO NOTHING
	`, glofoxUserID, w.branchID, w.studioID)
	if err != nil {
		return false, fmt.Errorf("insert log: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
