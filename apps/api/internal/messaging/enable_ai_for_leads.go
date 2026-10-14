package messaging

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/studios"
)

type enableAIForLeadsReq struct {
	LeadIDs []string `json:"leadIds"`
}

// EnableAIForLeadsHandler godoc
//
//	@Summary		Enable AI for specific, admin-selected leads
//	@Description	Turns AI on for exactly the leads passed in leadIds, regardless of status or whether they've already been contacted — unlike /leads/auto-contact/enable, which operates on whole lead statuses. A lead with no prior conversation gets the studio's normal greeting; a lead with existing (e.g. imported) conversation history gets a genuine, context-aware AI-composed follow-up instead. Every message goes through the normal paced outbound queue.
//	@Tags			Messaging - AI
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string					true	"Studio ID"
//	@Param			body		body		enableAIForLeadsReq	true	"Lead IDs"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or body"
//	@Router			/api/v1/studios/{studioId}/leads/enable-ai-for-selected [post]
func (w *AIWorker) EnableAIForLeadsHandler(rw http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(rw, r)
	if !ok {
		return
	}
	var req enableAIForLeadsReq
	if !httpx.DecodeJSON(rw, r, &req) {
		return
	}
	if len(req.LeadIDs) == 0 {
		httpx.WriteValidationError(rw, map[string]string{"leadIds": "at least one lead is required"})
		return
	}
	leadIDs := make([]uuid.UUID, 0, len(req.LeadIDs))
	for _, s := range req.LeadIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			httpx.WriteValidationError(rw, map[string]string{"leadIds": fmt.Sprintf("invalid lead id %q", s)})
			return
		}
		leadIDs = append(leadIDs, id)
	}
	enqueued, errs := w.EnableAIForLeads(r.Context(), studioID, leadIDs)
	status := http.StatusOK
	if enqueued == 0 && len(errs) > 0 {
		status = http.StatusUnprocessableEntity
	}
	httpx.JSON(rw, status, map[string]any{
		"enqueued": enqueued,
		"errors":   errs,
		"message":  fmt.Sprintf("Enabled AI for %d lead(s)", enqueued),
	})
}

// EnableAIForLeads turns AI on for a specific, admin-selected set of leads
// — as opposed to EnqueueAutoContactForLeadsByStatus's whole-status bulk
// action, this works on exactly the leads the admin picked, regardless of
// whether they've already been contacted. For each lead:
//
//   - No existing conversation history (brand-new lead, or one that was
//     never actually messaged): sends the studio's normal greeting, same
//     template/behavior as AutoContactWorker.processItem uses for a
//     freshly auto-contacted lead.
//   - Existing conversation with real message history already (most
//     commonly: a WhatsApp Web backfill-imported conversation that was
//     never auto-contacted, so ai_enabled is still false) — sending the
//     generic greeting here would be jarring ("nice to meet you" to
//     someone with months of prior chat) or silently useless (nothing
//     sent, AI just waits). Instead, the AI reads the actual history and
//     composes a genuine, context-aware follow-up picking back up on
//     whatever they last talked about.
//
// Either way, ai_enabled is turned on for the conversation afterward so a
// reply gets a real AI response. Every message this enqueues goes through
// the exact same outbound queue as any other send (OutboundWorker.run),
// which enforces the studio's configured WhatsApp send-spacing and daily
// message limit — not a separate, unpaced blast path.
func (w *AIWorker) EnableAIForLeads(ctx context.Context, studioID uuid.UUID, leadIDs []uuid.UUID) (enqueued int, errs map[string]string) {
	errs = map[string]string{}
	if len(leadIDs) == 0 {
		return 0, errs
	}

	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		errs["_"] = "studio not found"
		return 0, errs
	}

	// Same WhatsApp-channel fallback order as AutoContactWorker.processItem
	// (Meta → Web), resolved once since it's the same for every lead in
	// this studio — SMS is deliberately not a fallback here, unlike the
	// auto-contact path, since a context-aware follow-up built from
	// WhatsApp history belongs back on WhatsApp.
	channelKind := KindWhatsAppMeta
	if _, err := w.msgRepo.GetActiveChannelByKind(ctx, studioID, KindWhatsAppMeta); err != nil {
		if _, err := w.msgRepo.GetActiveChannelByKind(ctx, studioID, KindWhatsAppWeb); err == nil {
			channelKind = KindWhatsAppWeb
		} else {
			errs["_"] = "no active WhatsApp channel for this studio"
			return 0, errs
		}
	}

	for _, leadID := range leadIDs {
		if err := w.enableAIForOneLead(ctx, studioID, leadID, channelKind, studio); err != nil {
			errs[leadID.String()] = err.Error()
			continue
		}
		enqueued++
	}
	return enqueued, errs
}

func (w *AIWorker) enableAIForOneLead(ctx context.Context, studioID, leadID uuid.UUID, channelKind ChannelKind, studio *studios.Studio) error {
	lead, err := w.leadsRepo.GetLead(ctx, studioID, leadID)
	if err != nil {
		return fmt.Errorf("lead not found")
	}
	if lead.DNDEnabled {
		return fmt.Errorf("lead has Do Not Disturb enabled")
	}

	// Reuse the lead's existing conversation when it has one (e.g. a WhatsApp
	// Web backfill-imported thread) rather than re-deriving a contact value
	// from lead.Phone — WhatsApp Web inbound messages are stored under the
	// raw JID ("919945417287@c.us"), which differs from the plain-digits
	// value sanitizePhone produces, so creating fresh here would silently
	// open a second, empty conversation next to the one with real history.
	conv, err := w.msgRepo.GetConversationByLeadID(ctx, studioID, leadID)
	if err != nil {
		return fmt.Errorf("failed to look up existing conversation: %w", err)
	}
	if conv == nil {
		phone := sanitizePhone(lead.Phone)
		if phone == "" {
			return fmt.Errorf("lead has no phone number")
		}
		conv, err = w.msgSvc.CreateConversation(ctx, studioID, CreateConversationInput{
			ChannelKind:  channelKind,
			ContactValue: phone,
			DisplayName:  lead.Name,
			LeadID:       &leadID,
		})
		if err != nil {
			return fmt.Errorf("failed to open conversation: %w", err)
		}
	}

	// Cap at aiHistoryWindow-ish depth — enough real context for a good
	// follow-up without feeding an LLM prompt hundreds of messages long for
	// a heavily-imported contact.
	history, histErr := w.msgRepo.ListMessages(ctx, studioID, conv.ID, 40, nil)
	if histErr != nil {
		w.log.Warn("enable-ai-for-leads: failed to load history, treating as no prior history", "lead_id", leadID, "err", histErr)
		history = nil
	}

	var body string
	if len(history) > 0 {
		body = w.composeFollowUpFromHistory(ctx, studioID, studio, lead, history)
	}
	if strings.TrimSpace(body) == "" {
		// No history, or the LLM follow-up failed/came back empty — fall
		// back to the studio's normal greeting, same as a fresh auto-contact.
		template := studio.GreetingMessage
		if template == "" {
			template = defaultGreetingTemplate
		}
		body = renderGreeting(template, studio, *lead)
	}

	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: conv.ID,
		Body:           body,
		SourceKind:     SourceAI,
		SourceRef:      "enable_ai_manual",
	}); err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	// A lead whose auto-contact stage is still "bot owns this stage" (see
	// AIWorker.handleInbound's botOwnedStage check — anything other than "",
	// "completed", "awaiting_options" or "awaiting_interest") was never
	// actually advanced by AutoContactWorker, most commonly an imported/
	// manually-added lead that starts at the generic CreateLead default of
	// "initial" and was never auto-contacted. Sending a message here without
	// also moving the stage forward left such leads permanently stuck: the
	// very next customer reply would hit botOwnedStage and the AI worker
	// would silently defer to a stage machine that was never actually
	// running for this lead, leaving the conversation unanswered forever
	// despite ai_enabled being true. Advance it the same way
	// AutoContactWorker.processItem does after its own initial send.
	stage := lead.AutoContactStage
	botOwnedStage := stage != "" && stage != "completed" && stage != "awaiting_options" && stage != "awaiting_interest"
	if botOwnedStage {
		if err := w.leadsRepo.UpdateAutoContactStage(ctx, studioID, leadID, "awaiting_interest"); err != nil {
			w.log.Warn("enable-ai-for-leads: failed to advance auto-contact stage", "lead_id", leadID, "err", err)
		}
	}

	if err := w.msgRepo.SetConversationAIEnabled(ctx, studioID, conv.ID, true); err != nil {
		w.log.Warn("enable-ai-for-leads: failed to enable AI flag", "lead_id", leadID, "err", err)
	}
	return nil
}

// composeFollowUpFromHistory asks the LLM waterfall for an actual outbound
// message (not a silent internal summary like summarizeMessagesInto/
// ensureRollingSummary) — a natural WhatsApp-style follow-up that picks
// back up on this specific conversation, in the studio's own voice when one
// is configured. Returns "" (never an error) on any failure so the caller
// can fall back to the plain greeting instead of sending nothing or erroring
// the whole bulk action out over one studio's/lead's history.
func (w *AIWorker) composeFollowUpFromHistory(ctx context.Context, studioID uuid.UUID, studio *studios.Studio, lead *leads.Lead, history []Message) string {
	var b strings.Builder
	b.WriteString("You are writing a WhatsApp follow-up message on behalf of " + studio.Name + " to re-open a conversation that went quiet. ")
	b.WriteString("Below is the real prior conversation with this contact. Write ONE short, natural, friendly message picking back up on whatever they last talked about — reference it specifically, don't just say a generic \"hi, following up\". ")
	b.WriteString("Do not repeat the whole history back to them. Output only the message itself, no preamble, no quotes. ")
	b.WriteString("Never sign off with a staff member's name or any bracket placeholder like [Your Name] — you have no real staff name to use here. If you want to identify who's writing, use the studio name (" + studio.Name + ") instead, or just don't sign off at all.\n\n")
	if studio.CommunicationStyleProfile != "" {
		b.WriteString("House style for this studio's messages:\n" + studio.CommunicationStyleProfile + "\n\n")
	}
	b.WriteString("Conversation history:\n")
	for _, m := range history {
		speaker := "Customer"
		if m.Direction == DirectionOutbound {
			speaker = "Studio"
		}
		fmt.Fprintf(&b, "%s: %s\n", speaker, m.Body)
	}
	prompt := b.String()

	reply, sourceRef := llmWaterfall(ctx, w.studiosRepo, w.llmRepo, w.msgRepo, w.claude, w.claudeAPIURL, w.log, studioID, studio, prompt, w.answerCache, "enable_ai_followup")
	if reply == "" {
		w.log.Warn("enable-ai-for-leads: follow-up generation failed, falling back to greeting", "studio_id", studioID, "lead_id", lead.ID)
		return ""
	}
	w.log.Info("enable-ai-for-leads: composed context-aware follow-up", "studio_id", studioID, "lead_id", lead.ID, "source", sourceRef)
	reply = strings.TrimSpace(reply)
	reply = strings.ReplaceAll(reply, "{{lead_name}}", lead.Name)
	reply = strings.ReplaceAll(reply, "{{lead_first_name}}", lead.FirstName)
	reply = strings.ReplaceAll(reply, "{{studio_name}}", studio.Name)
	// Belt-and-suspenders against the prompt instruction above not being
	// followed: the model occasionally signs off with a bracket placeholder
	// it had no real value for (e.g. "[Your Name]") instead of a staff
	// member's actual name — caught in production. Swap any such
	// placeholder for the studio name rather than letting a literal
	// "[Your Name]" reach a real customer.
	reply = staffNamePlaceholderRe.ReplaceAllString(reply, studio.Name)
	return reply
}

// staffNamePlaceholderRe matches an unfilled "sign off with your name" style
// placeholder like "[Your Name]", "[Staff Name]", "[Name]" — case
// insensitive, tolerant of the common variants seen in the wild.
var staffNamePlaceholderRe = regexp.MustCompile(`(?i)\[\s*(your |staff |my )?name\s*\]`)
