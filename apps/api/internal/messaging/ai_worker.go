package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/projectx/api/internal/decisiontree"
	"github.com/projectx/api/internal/integrations/bifrost"
	"github.com/projectx/api/internal/integrations/embeddings"
	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/integrations/llmgateway"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging/prompts"
	"github.com/projectx/api/internal/studios"
)

const (
	aiResyncInterval = 30 * time.Second
	// aiPipelineVersion tags every ai_turns row (task 9.2/doc §8.9) with
	// which generation of the reply pipeline produced it — bump this
	// when a change to the structured-call/tools/guards pipeline would
	// make old and new turns not directly comparable in a report.
	aiPipelineVersion = "ai-reply-flow-llm-design-v1"
)

type AIWorker struct {
	bus           Bus
	msgRepo       *Repo
	msgSvc        *Service
	studiosRepo   *studios.Repo
	leadsRepo     *leads.Repo
	dtSvc         *decisiontree.Service
	bifrostClient *bifrost.Client
	embeddings    *embeddings.Client
	llmRepo       *llm.Repo
	llmGateway    *llmgateway.Client
	log           *slog.Logger
	subs          map[uuid.UUID]func()
	httpClient    *http.Client
	// glofoxClient serves the reply-tools capability (task 7.2) —
	// optional (nil if Glofox isn't configured for this deployment), set
	// via WithGlofoxClient after construction so NewAIWorker's existing
	// call sites don't all need updating for one new optional dependency.
	glofoxClient *glofox.Client

	// Debounce of rapid-fire inbound messages (so a customer sending
	// several messages back to back gets ONE reply addressing all of
	// them, not one per message) is durable since task 5.2 — see
	// debounceInboundMessage/RunDebouncePoll and the pending_ai_replies
	// table; no in-memory state is kept on this struct for it anymore.
}

// inboundDebounceWindow is how long AIWorker waits after an inbound message
// before actually generating a reply — reset on every further message from
// the same conversation within the window, so a burst of messages only ever
// produces one reply, generated after the customer has actually paused.
// Long enough to comfortably cover someone typing 2-3 messages in a row,
// short enough that a single message still gets a reply that doesn't feel
// delayed.
const inboundDebounceWindow = 6 * time.Second

func NewAIWorker(bus Bus, msgRepo *Repo, msgSvc *Service, studiosRepo *studios.Repo, leadsRepo *leads.Repo, dtSvc *decisiontree.Service, bifrostClient *bifrost.Client, embClient *embeddings.Client, llmRepo *llm.Repo, llmGateway *llmgateway.Client, log *slog.Logger) *AIWorker {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	return &AIWorker{
		bus:           bus,
		msgRepo:       msgRepo,
		msgSvc:        msgSvc,
		studiosRepo:   studiosRepo,
		leadsRepo:     leadsRepo,
		dtSvc:         dtSvc,
		bifrostClient: bifrostClient,
		embeddings:    embClient,
		llmRepo:       llmRepo,
		llmGateway:    llmGateway,
		log:           log,
		subs:          make(map[uuid.UUID]func()),
		httpClient:    client,
	}
}

// WithGlofoxClient sets the Glofox client the reply-tools capability
// (task 7.2) calls for check_availability/create_booking. Optional — a
// nil client (the default, if this is never called) makes
// executeToolCalls fail closed per tool-call attempt rather than panic.
func (w *AIWorker) WithGlofoxClient(c *glofox.Client) *AIWorker {
	w.glofoxClient = c
	return w
}

func (w *AIWorker) Run(ctx context.Context) {
	w.log.Info("ai worker started")
	// Subscribe immediately so messages arriving in the first 30s are not missed.
	w.syncSubscriptions(ctx)
	t := time.NewTicker(aiResyncInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("ai worker stopping")
			// unsubscribe
			for _, u := range w.subs {
				u()
			}
			return
		case <-t.C:
			w.syncSubscriptions(ctx)
		}
	}
}

func (w *AIWorker) syncSubscriptions(ctx context.Context) {
	list, err := w.studiosRepo.List(ctx)
	if err != nil {
		w.log.Error("list studios for ai worker", "err", err)
		return
	}
	// subscribe to any new studios
	for _, s := range list {
		if _, ok := w.subs[s.ID]; ok {
			continue
		}
		ch, unsub := w.bus.Subscribe(s.ID)
		w.subs[s.ID] = unsub
		go w.listenStudio(ctx, s.ID, ch) // #nosec G118 -- already passes ctx (the worker's own long-lived run context, appropriate for a listener meant to live as long as the worker does), not context.Background()
		w.log.Info("ai worker subscribed to studio", "studio", s.ID)
	}
}

func (w *AIWorker) listenStudio(ctx context.Context, studioID uuid.UUID, ch <-chan Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			switch evt.Kind {
			case EvtMessageReceived:
				if evt.MessageID == nil {
					continue
				}
				w.debounceInboundMessage(ctx, studioID, evt.ConversationID, *evt.MessageID)
			case EvtMessageSent:
				// Cancel a pending debounced reply on ANY staff message in
				// this conversation (ai-reply-flow-llm-design task 5.3) —
				// not just a DND trigger, which is all the old behavior
				// covered (see handleMessage's DND branch, which only
				// cancels already-scheduled outbound jobs, never the
				// debounce window itself).
				if evt.MessageID == nil {
					continue
				}
				go w.cancelPendingReplyIfStaffSent(context.WithoutCancel(ctx), studioID, evt.ConversationID, *evt.MessageID)
			case EvtWAWebBackfillDone:
				if evt.ChannelAccountID == nil {
					continue
				}
				go func(channelAccountID uuid.UUID) {
					if err := w.summarizeConversationsAfterBackfill(ctx, studioID, channelAccountID); err != nil {
						w.log.Error("ai summarize conversations after backfill", "err", err)
					}
				}(*evt.ChannelAccountID)
			}
		}
	}
}

// debounceInboundMessage replaces an immediate per-message handleMessage
// call with a short, per-conversation quiet window (inboundDebounceWindow):
// each new inbound message from the same conversation resets the window
// instead of firing its own reply, so a customer sending several messages
// back to back ("hi", "I mean Tuesday", "is there a discount?") gets exactly
// ONE reply once they've actually paused, addressing everything in the
// burst — not one reply per message. handleMessage itself needs no changes:
// by the time the window elapses, every message in the burst is already
// persisted, so the single handleMessage call (anchored on the LATEST
// message ID seen) loads them all as ordinary conversation history.
//
// Durable since ai-reply-flow-llm-design task 5.2: this writes a
// pending_ai_replies row instead of starting an in-memory time.AfterFunc,
// so a process restart during the window doesn't drop the pending reply —
// RunDebouncePoll (started once at process startup, see main.go) is what
// actually fires it once fire_at elapses, from any process that happens
// to be running the poll loop when that time comes.
func (w *AIWorker) debounceInboundMessage(ctx context.Context, studioID, conversationID, messageID uuid.UUID) {
	fireAt := time.Now().Add(inboundDebounceWindow)
	if err := w.msgRepo.UpsertPendingAIReply(ctx, conversationID, studioID, messageID, fireAt); err != nil {
		w.log.Error("durable debounce: upsert pending ai reply failed", "conversation_id", conversationID, "err", err)
	}
}

// cancelPendingReplyIfStaffSent deletes conversationID's pending debounced
// reply when messageID turns out to be a staff-authored send (task 5.3).
// Checked here rather than filtered at the event-subscription level since
// EvtMessageSent doesn't carry SourceKind itself — only GetMessageByID
// does, same pattern style_worker.go's learnFromMessageIfStaffReply uses
// for the identical "is this actually a human, not AI/automation" check.
func (w *AIWorker) cancelPendingReplyIfStaffSent(ctx context.Context, studioID, conversationID, messageID uuid.UUID) {
	msg, err := w.msgRepo.GetMessageByID(ctx, studioID, messageID)
	if err != nil || msg == nil || msg.SourceKind != SourceStudioUser {
		return
	}
	if err := w.msgRepo.DeletePendingAIReply(ctx, conversationID); err != nil {
		w.log.Warn("durable debounce: cancel on staff message failed", "conversation_id", conversationID, "err", err)
	}
}

// debouncePollInterval is how often RunDebouncePoll checks for a pending
// reply whose fire_at has elapsed — short relative to inboundDebounceWindow
// (6s) so the actual send latency this adds is small, but not so short it
// turns into a busy-poll; matches this package's other poll workers' "cheap
// query when nothing is due" pattern (style_worker.go/worker.go), just at a
// tighter interval since this one is latency-sensitive.
const debouncePollInterval = 1 * time.Second

// RunDebouncePoll is the durable counterpart to the old in-memory timer —
// started once at process startup (main.go), it fires every pending reply
// whose debounce window has elapsed. A process restart mid-window simply
// means the row is picked up by whichever process's poll loop next ticks
// past fire_at, instead of being lost (ai-reply-flow-llm-design task 5.2).
func (w *AIWorker) RunDebouncePoll(ctx context.Context) {
	w.log.Info("ai debounce poll worker started", "poll_interval", debouncePollInterval)
	t := time.NewTicker(debouncePollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("ai debounce poll worker stopping")
			return
		case <-t.C:
			w.fireDuePendingReplies(ctx)
		}
	}
}

func (w *AIWorker) fireDuePendingReplies(ctx context.Context) {
	due, err := w.msgRepo.ClaimDuePendingAIReplies(ctx, time.Now())
	if err != nil {
		w.log.Error("durable debounce: claim due pending ai replies failed", "err", err)
		return
	}
	for _, p := range due {
		go w.fireOnePendingReply(ctx, p)
	}
}

func (w *AIWorker) fireOnePendingReply(ctx context.Context, p PendingAIReply) {
	// Without recover(), a panic anywhere in handleMessage kills this
	// goroutine silently — no error logged, no reply sent, no trace.
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("ai handle message panicked", "message_id", p.MessageID, "studio_id", p.StudioID,
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	if err := w.handleMessage(ctx, p.StudioID, p.MessageID); err != nil {
		w.log.Error("ai handle message", "err", err)
	}
	if err := w.msgRepo.DeletePendingAIReply(ctx, p.ConversationID); err != nil {
		w.log.Warn("durable debounce: cleanup after firing failed", "conversation_id", p.ConversationID, "err", err)
	}
}

// summarizeConversationsAfterBackfill generates and stores a short internal
// AI summary of each conversation's recent history for a studio's
// whatsapp_web channel, right after a chat-history import completes. The
// summary is never shown to the admin — it's folded into buildPrompt as
// extra context so the first live reply after connecting isn't blind to
// everything that was discussed before.
func (w *AIWorker) summarizeConversationsAfterBackfill(ctx context.Context, studioID, channelAccountID uuid.UUID) error {
	convIDs, err := w.msgRepo.ListConversationIDsByChannel(ctx, studioID, channelAccountID)
	if err != nil {
		return fmt.Errorf("list conversations for summarization: %w", err)
	}
	if len(convIDs) == 0 {
		return nil
	}

	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return fmt.Errorf("load studio for summarization: %w", err)
	}

	w.log.Info("ai: summarizing conversations after wa-web backfill", "studio_id", studioID, "conversations", len(convIDs))
	for _, convID := range convIDs {
		if err := w.summarizeConversation(ctx, studioID, convID, studio); err != nil {
			w.log.Warn("ai: summarize conversation failed, continuing", "studio_id", studioID, "conversation_id", convID, "err", err)
		}
	}
	return nil
}

// summarizeConversation fetches the last 10 messages of a conversation and
// asks the same LLM waterfall used for replies to produce a 2-3 sentence
// summary, stored for later use as AI reply context. No-ops (with a log
// line, matching the reply-generation silent-failure pattern) if no LLM
// provider is configured for the studio.
func (w *AIWorker) summarizeConversation(ctx context.Context, studioID, convID uuid.UUID, studio *studios.Studio) error {
	history, err := w.msgRepo.ListMessages(ctx, studioID, convID, 10, nil)
	if err != nil {
		return fmt.Errorf("fetch message history: %w", err)
	}
	return w.summarizeMessagesInto(ctx, studioID, convID, studio, history)
}

// summarizeMessagesInto builds a summarization prompt from an arbitrary message slice,
// runs it through the LLM waterfall, and stores the result. Shared by summarizeConversation
// (backfill) and ensureRollingSummary (live conversations).
func (w *AIWorker) summarizeMessagesInto(ctx context.Context, studioID, convID uuid.UUID, studio *studios.Studio, history []Message) error {
	if len(history) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("Summarize this WhatsApp conversation history in 2-3 short sentences, focused on what the customer wants and where things were left off. Be concise and factual — no greeting, no filler.\n\n")
	for _, m := range history {
		speaker := "Customer"
		if m.Direction == DirectionOutbound {
			speaker = "Studio"
		}
		fmt.Fprintf(&b, "%s: %s\n", speaker, m.Body)
	}
	prompt := b.String()

	summary, sourceRef := w.runSummaryWaterfall(ctx, studioID, studio, prompt)
	if summary == "" {
		w.log.Warn("ai: skipping conversation summary, all providers failed or not configured", "studio_id", studioID, "conversation_id", convID)
		return nil
	}
	w.log.Info("ai: conversation summarized", "studio_id", studioID, "conversation_id", convID, "source", sourceRef)
	return w.msgRepo.SetConversationAISummary(ctx, studioID, convID, strings.TrimSpace(summary))
}

// runSummaryWaterfall mirrors the reply-generation waterfall in handleMessage
// (Groq -> Gemini -> Claude) but is factored out standalone since
// summarization has no Message/decision-tree/KB context to gather.
func (w *AIWorker) runSummaryWaterfall(ctx context.Context, studioID uuid.UUID, studio *studios.Studio, prompt string) (text string, sourceRef string) {
	return llmWaterfall(ctx, w.llmGateway, studioID, studio, prompt, llmgateway.FeatureSummary)
}

// aiHistoryWindow is how many recent messages handleMessage loads as the model's
// immediate context. rollingSummaryRefreshEvery bounds how often ensureRollingSummary
// re-summarizes a long conversation (every Nth message past the window) instead of on
// every turn.
const (
	aiHistoryWindow            = 15
	rollingSummaryRefreshEvery = 5
)

// ensureRollingSummary keeps ai_context_summary caught up for a live conversation once it
// grows past aiHistoryWindow, so older turns aren't simply lost once they fall out of the
// recency window. Best-effort and fire-and-forget — a failed or skipped refresh just means
// the next reply falls back to the recency window alone.
func (w *AIWorker) ensureRollingSummary(ctx context.Context, studioID uuid.UUID, convID uuid.UUID, studio *studios.Studio) {
	// Capped at 500 (ListMessages' own max) so the "every Nth message" gate below keeps
	// working for a very long-running conversation instead of freezing at a smaller cap.
	full, err := w.msgRepo.ListMessages(ctx, studioID, convID, 500, nil)
	if err != nil {
		w.log.Warn("ai: rolling summary — failed to load full history", "conversation_id", convID, "err", err)
		return
	}
	if !shouldRefreshRollingSummary(len(full)) {
		return
	}
	older := full[:len(full)-aiHistoryWindow]
	if err := w.summarizeMessagesInto(ctx, studioID, convID, studio, older); err != nil {
		w.log.Warn("ai: rolling summary failed", "conversation_id", convID, "err", err)
	}
}

// shouldRefreshRollingSummary reports whether to (re-)summarize: never while everything
// fits in the window, then once on first crossing it, then every rollingSummaryRefreshEvery
// messages after that.
func shouldRefreshRollingSummary(total int) bool {
	if total <= aiHistoryWindow {
		return false
	}
	return total == aiHistoryWindow+1 || total%rollingSummaryRefreshEvery == 0
}

// llmWaterfall is a thin wrapper around the LLM gateway
// (internal/integrations/llmgateway) shared by this package's background
// AI callers (summary, style_profile, test_chat, template_gen).
//
// No longer Redis-cache-first (ai-reply-flow-llm-design task 8.1): the
// AnswerCache this used to check/populate was torn down per the design
// doc's §9 and an explicit user decision recorded in this change's
// proposal.md, since a cached answer can't carry the per-call facts/
// guards the new structured-reply pipeline (task 3+) requires — every
// call now genuinely hits the gateway.
func llmWaterfall(ctx context.Context, gw *llmgateway.Client, studioID uuid.UUID, studio *studios.Studio, prompt string, feature llmgateway.Feature) (text string, sourceRef string) {
	// Superadmin-only master switch — a studio with this off never gets any
	// AI call through this shared waterfall (summary, style_profile,
	// test_chat, template_gen, enable_ai_followup), regardless of which
	// keys/models are centrally configured.
	if studio != nil && !studio.AIAccessEnabled {
		return "", ""
	}
	var err error
	text, sourceRef, err = gw.Complete(ctx, feature, studioID, prompt)
	if err != nil {
		// Only returned for a missing ai_feature_routes row — a config
		// bug (every feature this function serves is seeded by migration
		// 20261029090004), not a provider failure, so it's worth a loud
		// log even though callers here only ever check text=="".
		slog.Default().Error("llm gateway: no route configured for feature", "feature", feature, "err", err)
		return "", ""
	}
	return text, sourceRef
}

// bifrostAttemptProvider, reasoning-block stripping, auth-failure
// detection, and failing-key flagging all moved to
// internal/integrations/llmgateway (see that package's gateway.go) once
// handleMessage's inline customer_reply waterfall — their last caller —
// was migrated to llmGateway.Complete.

// analyzeSentiment returns: sentiment (-1=negative, 0=neutral, 1=positive), confidence score, and detected keywords
func (w *AIWorker) analyzeSentiment(text string) (int, float64, []string) {
	text = strings.ToLower(text)

	positiveKeywords := []string{"yes", "interested", "great", "love", "good", "perfect", "thanks", "thank you", "definitely", "sure", "count me in", "sign me up", "book it"}
	negativeKeywords := []string{"no", "not interested", "bad", "hate", "no thanks", "never", "not now", "maybe later", "skip", "cancel"}

	var detectedKeywords []string
	positiveScore := 0.0
	negativeScore := 0.0

	for _, kw := range positiveKeywords {
		if strings.Contains(text, kw) {
			positiveScore += 1.0
			detectedKeywords = append(detectedKeywords, kw)
		}
	}

	for _, kw := range negativeKeywords {
		if strings.Contains(text, kw) {
			negativeScore += 1.0
			detectedKeywords = append(detectedKeywords, kw)
		}
	}

	sentiment := 0
	confidence := 0.0

	if positiveScore > negativeScore {
		sentiment = 1
		confidence = positiveScore / (positiveScore + negativeScore + 1)
	} else if negativeScore > positiveScore {
		sentiment = -1
		confidence = negativeScore / (positiveScore + negativeScore + 1)
	} else {
		confidence = 0.5
	}

	return sentiment, confidence, detectedKeywords
}

// stopKeywords are opt-out words a lead can reply with to end all automated
// contact. Matched as the entire trimmed message (case-insensitive) so
// ordinary sentences containing "stop" (e.g. "stop by later") don't trigger it.
var stopKeywords = map[string]bool{
	"stop":        true,
	"unsubscribe": true,
	"opt out":     true,
	"optout":      true,
}

func isStopMessage(body string) bool {
	normalized := strings.ToLower(strings.TrimSpace(body))
	normalized = strings.Trim(normalized, ".!? ")
	return stopKeywords[normalized]
}

func (w *AIWorker) handleMessage(ctx context.Context, studioID uuid.UUID, messageID uuid.UUID) error {
	msg, err := w.msgRepo.GetMessageByID(ctx, studioID, messageID)
	if err != nil {
		return fmt.Errorf("fetch message: %w", err)
	}
	if msg == nil {
		return nil
	}
	// Only respond to customer inbound messages
	if msg.Direction != DirectionInbound || msg.SourceKind != SourceCustomer {
		return nil
	}

	// Get conversation context
	conv, err := w.msgRepo.GetConversation(ctx, studioID, msg.ConversationID)
	if err != nil {
		return fmt.Errorf("fetch conversation: %w", err)
	}
	if conv == nil {
		return nil
	}

	// Get channel details to check if we should reply
	channel, err := w.msgRepo.GetChannelByID(ctx, studioID, conv.ChannelAccountID)
	if err != nil {
		return fmt.Errorf("fetch channel: %w", err)
	}
	if channel == nil {
		return nil
	}

	// Get lead associated with this conversation (if any)
	var lead *leads.Lead
	if conv.LeadID != nil {
		lead, err = w.leadsRepo.GetLead(ctx, studioID, *conv.LeadID)
		if err != nil {
			w.log.Error("fetch lead for ai context", "err", err)
		}
	}

	// A genuine reply means the "no reply" follow-up cascade no longer
	// applies to this lead — cancel any of its still-pending steps. Runs
	// regardless of ai_enabled/DND below, since even a lead being handled
	// manually (or opting out) has, by definition, replied.
	if lead != nil {
		if _, err := w.msgRepo.CancelPendingFollowupJobsForLead(ctx, studioID, lead.ID); err != nil {
			w.log.Error("failed to cancel pending followup jobs", "lead", lead.ID, "err", err)
		}
	}

	// AI auto-reply is off for this conversation — studio user hasn't enabled it yet.
	if !conv.AIEnabled {
		w.log.Info("ai worker: ai_enabled=false — no reply sent", "conversation", conv.ID)
		return nil
	}

	// Do Not Disturb: either the lead/conversation already opted in to DND
	// (manually, or from a previous "stop"), or this message is itself an
	// opt-out keyword. Either way: enable DND, cancel every pending scheduled
	// message for this conversation, and never send a reply. Pipeline status
	// is left untouched. Conversations with no linked lead (e.g. imported
	// WhatsApp Web contacts) carry DND on conversations.dnd_enabled instead —
	// see SetConversationDNDEnabled.
	if (lead != nil && lead.DNDEnabled) || conv.DNDEnabled || isStopMessage(msg.Body) {
		if lead != nil && !lead.DNDEnabled {
			if err := w.leadsRepo.SetDNDEnabled(ctx, studioID, lead.ID, true); err != nil {
				w.log.Error("stop keyword: failed to enable dnd", "lead", lead.ID, "err", err)
			}
		} else if lead == nil && !conv.DNDEnabled {
			if err := w.msgRepo.SetConversationDNDEnabled(ctx, studioID, conv.ID, true); err != nil {
				w.log.Error("stop keyword: failed to enable conversation dnd", "conversation", conv.ID, "err", err)
			}
		}
		if _, err := w.msgRepo.CancelPendingJobsForConversation(ctx, studioID, conv.ID); err != nil {
			w.log.Error("dnd: failed to cancel pending jobs", "conversation", conv.ID, "err", err)
		}
		w.log.Info("ai worker: dnd active — no reply sent", "conversation", conv.ID, "lead_id", conv.LeadID)
		return nil
	}

	if lead != nil {
		// Skip AI only for mid-flow stages where the bot owns the conversation
		// (date/time selection, plan selection, reason collection).
		// At awaiting_interest and awaiting_options the bot only reacts to exact
		// numeric/keyword choices; anything else (questions, greetings) falls to AI.
		stage := lead.AutoContactStage
		w.log.Info("ai worker: lead stage check", "lead_id", lead.ID, "stage", stage, "lead_status", string(lead.Status), "message", msg.Body)
		botOwnedStage := stage != "" &&
			stage != "completed" &&
			stage != "awaiting_options" &&
			stage != "awaiting_interest"
		if botOwnedStage {
			w.log.Info("ai worker: skipping — bot owns this stage", "lead", lead.ID, "stage", stage)
			return nil
		}
		// At awaiting_options / awaiting_interest / no-stage: skip only if the bot
		// already queued an outbound reply for this specific inbound message.
		// Checks any non-customer source, not just 'automation' — the auto-contact
		// stage machine's SendTrialPaymentLink (triggered from here for exact "1"/
		// trial replies during awaiting_options) enqueues with source_kind 'ai', so
		// filtering on 'automation' alone let this same reply through twice.
		var botReplied bool
		_ = w.msgRepo.Pool().QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM outbound_jobs
				WHERE conversation_id = $1 AND source_kind IN ('automation', 'ai')
				AND created_at >= $2
			)
		`, conv.ID, msg.CreatedAt).Scan(&botReplied)
		w.log.Info("ai worker: bot_replied check", "lead_id", lead.ID, "bot_replied", botReplied, "msg_created_at", msg.CreatedAt)
		if botReplied {
			w.log.Info("ai worker: skipping — automation already replied", "lead", lead.ID)
			return nil
		}
	}

	// Get studio to access knowledge base and timezone
	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return fmt.Errorf("fetch studio for ai context: %w", err)
	}

	// Superadmin-only master switch — separate from conv.AIEnabled above.
	// A studio with this off never gets AI replies, regardless of
	// per-conversation state or which keys/models are centrally configured.
	if !studio.AIAccessEnabled {
		w.log.Info("ai worker: studio ai_access_enabled=false — no reply sent", "studio_id", studioID)
		return nil
	}

	// Delay the AI's reply by a configurable number of seconds so it doesn't
	// look robotically instant. Only applies to replies within an
	// already-started conversation — the first outreach to a brand-new lead
	// uses initial_contact_delay_minutes instead (see autocontact_worker.go).
	replyDelay := time.Duration(0)
	if secs, dErr := w.studiosRepo.GetAIReplyDelaySeconds(ctx, studioID); dErr == nil {
		replyDelay = time.Duration(secs) * time.Second
	}

	// Fetch active plans for this studio
	plans, err := w.msgRepo.ListActivePlans(ctx, studioID)
	if err != nil {
		w.log.Error("fetch plans for ai context failed", "err", err)
		plans = []Plan{} // fallback to empty
	}

	// Active offers (ai-reply-flow-llm-design task 2.5) — same
	// fail-open-to-empty treatment as plans above: a lookup failure
	// should degrade the prompt, not block the reply.
	offers, err := w.msgRepo.ListActiveOffers(ctx, studioID)
	if err != nil {
		w.log.Error("fetch offers for ai context failed", "err", err)
		offers = []Offer{}
	}

	// Keyword sentiment as initial fallback
	sentiment, confidence, keywords := w.analyzeSentiment(msg.Body)

	// Gemini's centrally-configured key (superadmin AI Provider Settings) —
	// shared by the gate below (classification/expansion/rerank only run if
	// Gemini is configured, same pre-existing gate as before this
	// migration — the "intent_classify"/"query_expand"/"rerank" routes
	// themselves fall back to Groq internally via llmGateway, but that
	// fallback only ever kicks in once this gate has already let the call
	// through).
	apiKey, _ := w.studiosRepo.GetPlatformAIProviderKey(ctx, "gemini")

	// Recent-message window for the model's immediate context; ensureRollingSummary
	// (below) covers anything further back so a long conversation doesn't lose it.
	history, err := w.msgRepo.ListMessages(ctx, studioID, conv.ID, aiHistoryWindow, nil)
	if err != nil {
		w.log.Error("fetch message history for ai context failed", "err", err)
		history = []Message{*msg}
	}

	// Reply gates (ai-reply-flow-llm-design task 6) — on top of the
	// AI-off/DND/bot-flow gates already checked above. Same "silent to the
	// customer, visible to staff" treatment as the AIEnabled/DND gates
	// right above: no reply is sent (none of these warrant yet another
	// automated "someone will help you" message — staff-active literally
	// means staff is already there), but a platform_alerts row keeps this
	// from being silent to the team, per the doc's "never silent" principle
	// applied to staff visibility rather than customer messaging.
	if reason := w.checkReplyGates(ctx, studioID, conv, studio, history); reason != "" {
		w.log.Info("ai worker: reply gate blocked this reply", "studio_id", studioID, "conversation_id", conv.ID, "reason", reason)
		w.msgRepo.RecordAlert(ctx, "reply_gate_"+reason, map[string]any{
			"conversation_id": conv.ID,
			"message_id":      msg.ID,
		}, studioID)
		return nil
	}

	go func() { // best-effort, off the critical path
		w.ensureRollingSummary(context.WithoutCancel(ctx), studioID, conv.ID, studio)
	}()

	// Send greeting on the very first inbound message of a new conversation.
	// "First" = only one message in history (the current one) and a greeting is configured.
	if len(history) == 1 && studio.GreetingMessage != "" {
		greetingBody := studio.GreetingMessage
		// Substitute the same placeholders supported in reply templates.
		greetingBody = strings.ReplaceAll(greetingBody, "{{studio_name}}", studio.Name)
		if lead != nil {
			greetingBody = strings.ReplaceAll(greetingBody, "{{lead_name}}", lead.Name)
			greetingBody = strings.ReplaceAll(greetingBody, "{{lead_first_name}}", lead.FirstName)
			greetingBody = strings.ReplaceAll(greetingBody, "{{lead_status}}", string(lead.Status))
		}
		// Guaranteed AI-identity disclosure — appended in code, not left to
		// the studio's editable greeting text or the LLM's own judgment, so
		// every new conversation discloses this exactly once regardless of
		// how the greeting is customized.
		greetingBody += fmt.Sprintf("\n\n_You're chatting with %s's AI assistant — a real team member can jump in anytime you'd like._", studio.Name)
		if _, sendErr := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
			StudioID:       studioID,
			ConversationID: conv.ID,
			Body:           greetingBody,
			SourceKind:     SourceAI,
			SourceRef:      "greeting",
			ScheduledFor:   time.Now().UTC().Add(replyDelay),
		}); sendErr != nil {
			w.log.Error("failed to enqueue greeting message", "err", sendErr, "conv_id", conv.ID)
		} else {
			w.bus.Publish(ctx, Event{
				Kind:     EvtOutboundJobEnqueued,
				StudioID: studioID,
			})
			// Greeting is the only reply for a brand-new conversation.
			// Stop here — don't also fire the decision tree or AI.
			return nil
		}
	}

	var kbChunks []string
	var styleExamples []StyleExample
	var intent string
	// kbConfident tracks whether retrieval found high-confidence chunks.
	// Used to gate hallucination: if false, the prompt instructs the AI not to guess.
	kbConfident := false
	// retrievalRan is true once the knowledge-base retrieval pipeline actually ran for this
	// message (it needs a Gemini key). kbConfident is only meaningful when it did.
	retrievalRan := false

	lowerBody := strings.ToLower(msg.Body)

	// Explicit human-handoff request — a customer asking for a person takes
	// priority over everything else: escalate immediately, bypassing the
	// decision tree and AI reply entirely, regardless of intent
	// classification or KB confidence. Plain keyword match (not the
	// Gemini-based intent classifier) so it works even when a studio only
	// has Groq configured, and so it can short-circuit before any LLM call.
	humanRequestPhrases := []string{
		"talk to a human", "talk to a person", "talk to someone",
		"speak to a human", "speak to a person", "speak to someone",
		"speak with a human", "speak with a person", "speak with someone",
		"talk to a manager", "talk to your manager", "talk to the manager",
		"speak to a manager", "speak to your manager", "speak to the manager",
		"speak with a manager", "real person", "human agent", "human support",
		"customer service rep", "connect me to a", "connect me with a",
	}
	for _, phrase := range humanRequestPhrases {
		if !strings.Contains(lowerBody, phrase) {
			continue
		}
		w.log.Info("ai worker: explicit human-handoff request detected, escalating",
			"studio_id", studioID, "conversation_id", conv.ID, "message_id", msg.ID)
		if err := w.msgSvc.EscalateAndNotify(ctx, studioID, conv.ID, "Customer asked to speak with a human"); err != nil {
			w.log.Warn("failed to mark conversation escalated (explicit request)", "studio_id", studioID, "err", err)
			break
		}
		handoffBody := "Of course — connecting you with a real team member now. They'll be with you shortly!"
		if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
			StudioID:       studioID,
			ConversationID: conv.ID,
			Body:           handoffBody,
			SourceKind:     SourceAI,
			SourceRef:      "explicit_handoff_request",
			ScheduledFor:   time.Now().UTC().Add(replyDelay),
		}); err != nil {
			w.log.Error("failed to enqueue explicit handoff message", "err", err, "conv_id", conv.ID)
		} else {
			w.bus.Publish(ctx, Event{
				Kind:           EvtOutboundJobEnqueued,
				StudioID:       studioID,
				ConversationID: conv.ID,
			})
		}
		return nil
	}

	// A complaint, negative feedback or a bad-review threat goes to a person right away,
	// before any other handling — the AI must not reply to, argue with or smooth over an
	// unhappy customer. Works without any API key (the classifier adds a second check below).
	if isComplaintOrNegativeFeedback(msg.Body) {
		w.log.Info("ai worker: complaint or negative feedback detected, escalating",
			"studio_id", studioID, "conversation_id", conv.ID)
		w.escalateWithHandoff(ctx, studioID, conv.ID,
			"Customer complaint or negative feedback",
			complaintHandoffBody, "complaint_handoff", replyDelay)
		return nil
	}

	// Keyword override: runs before LLM classification so it works even when Gemini is not configured.
	// "trail" is a typo for "trial" that LLMs classify as hiking/off-topic.
	if strings.Contains(lowerBody, "trail") ||
		strings.Contains(lowerBody, "book trial") || strings.Contains(lowerBody, "book a trial") {
		intent = "booking_inquiry"
		w.log.Debug("intent set to booking_inquiry by keyword", "message_id", msg.ID)
	} else if strings.Contains(lowerBody, "become a member") || strings.Contains(lowerBody, "become member") ||
		strings.Contains(lowerBody, "want to join") || strings.Contains(lowerBody, "sign me up") ||
		strings.Contains(lowerBody, "want membership") || strings.Contains(lowerBody, "get membership") {
		intent = "ready_to_buy"
		w.log.Debug("intent set to ready_to_buy by keyword", "message_id", msg.ID)
	}

	if apiKey != "" && msg.Body != "" {
		retrievalRan = true
		// Step 1: embed the raw customer message directly for retrieval —
		// no separate LLM query-expansion call. Intent/sentiment/language
		// classification for prompt-shaping no longer happens here either;
		// the single structured reply call below reads the raw message
		// itself and classifies inline (ai-reply-flow-llm-design: collapsed
		// to satisfy the structured-reply spec's "exactly one LLM call for
		// intent/expansion/rerank/reply combined" requirement). `intent`
		// here keeps whatever the free keyword overrides above set it to
		// (or "" if none matched) — used only for the decision-tree/
		// complaint-escalation checks below, not for prompt-shaping.
		queryVec, err := w.embeddings.EmbedQuery(ctx, msg.Body)
		if err == nil {
			// Step 1b: Persist original message embedding fully async — not needed for this response.
			capturedIntent, capturedSentiment, capturedConf := intent, sentiment, confidence
			go func() {
				saveCtx := context.WithoutCancel(ctx)
				// PASSAGE, not query: msg.Body is being stored here as a future
				// search target (see SearchSemanticHistory), same as knowledge-base
				// chunks and staff replies. On error this falls back to queryVec
				// (a QUERY-space vector) — a pre-existing vector-space mismatch in
				// this fallback path, not introduced by this change; left as-is.
				vec, origErr := w.embeddings.EmbedPassage(saveCtx, msg.Body)
				if origErr != nil {
					vec = queryVec
				}
				if saveErr := w.msgRepo.SaveMessageEmbedding(saveCtx, msg.ID, vec, capturedIntent, capturedSentiment, float32(capturedConf)); saveErr != nil {
					w.log.Warn("save message embedding failed", "message_id", msg.ID, "err", saveErr)
				}
			}()

			// Step 2 + style-example retrieval in parallel. Semantic history
			// (SearchSemanticHistory) removed from this fan-out — see
			// ai-reply-flow-llm-design task 3.3's removal note at buildPrompt.
			type kbResult struct {
				chunks    []string
				confident bool
			}
			kbCh := make(chan kbResult, 1)
			styleCh := make(chan []StyleExample, 1)

			go func() {
				// Hybrid retrieval (8 candidates), already ordered by RRF
				// fusion relevance — take the top 4 directly. No separate LLM
				// rerank call (ai-reply-flow-llm-design: collapsed into the
				// single structured reply call, see structured_reply.go).
				matched, searchErr := w.studiosRepo.SearchKnowledgeChunksHybrid(ctx, studioID, queryVec, msg.Body, "all", 8)
				if searchErr != nil || len(matched) == 0 {
					if searchErr != nil {
						w.log.Warn("failed to search knowledge chunks", "studio_id", studioID, "err", searchErr)
					}
					kbCh <- kbResult{}
					return
				}
				topN := 4
				if len(matched) < topN {
					topN = len(matched)
				}
				top := make([]string, topN)
				for i := 0; i < topN; i++ {
					top[i] = matched[i].Content
				}
				kbCh <- kbResult{chunks: top, confident: len(top) >= 2}
			}()

			go func() {
				// Studio-wide (not conversation-scoped) search over the studio's
				// own past staff-authored replies — real examples of how THIS
				// studio has handled a similar situation before, not just tone.
				examples, styleErr := w.msgRepo.SearchStyleExamples(ctx, studioID, queryVec, 3)
				if styleErr != nil {
					w.log.Warn("failed to search style examples", "studio_id", studioID, "err", styleErr)
					styleCh <- nil
					return
				}
				// Lower bar than SearchSemanticHistory's 0.75 above: that's
				// matching a customer message against other customer messages
				// (near-paraphrases score high), this is matching a customer's
				// question against a staff reply answering a DIFFERENT question
				// — a genuinely relevant pair (verified against real studio data:
				// "can i check on the trial" → "How are you enjoying your trial
				// so far?" scored 0.673) naturally lands lower since question and
				// answer aren't textually similar the way two questions are. 0.75
				// here filtered out every real match in that same test.
				var filtered []StyleExample
				for _, ex := range examples {
					if ex.Score >= 0.6 {
						filtered = append(filtered, ex)
					}
				}
				styleCh <- filtered
			}()

			kb := <-kbCh
			kbChunks = kb.chunks
			kbConfident = kb.confident
			if len(kbChunks) > 0 {
				w.log.Info("rag pipeline complete", "studio_id", studioID, "final", len(kbChunks), "confident", kbConfident)
			}

			if styleResults := <-styleCh; len(styleResults) > 0 {
				styleExamples = styleResults
				w.log.Info("retrieved style examples", "studio_id", studioID, "count", len(styleExamples))
			}
		} else {
			w.log.Warn("failed to get message embedding for rag", "studio_id", studioID, "err", err)
		}
	}

	// A bare numeric reply ("1"/"2") to one of the platform's own booking-flow
	// menus (trial/member, then become-a-member yes/no) must be interpreted
	// against whatever menu was most recently sent — not against an unrelated
	// studio-authored decision tree node that happens to also key on "1"/"2"
	// (e.g. a pricing FAQ tree). Without this, a customer answering "become a
	// member?" with "2" could get matched to a stale tree node instead of
	// "not right now", derailing the conversation. Skip the tree whenever our
	// own option detector has a definitive read for the lead's current status.
	skipTreeForOptionReply := false
	if lead != nil {
		if _, ok := w.detectOptionChoice(msg.Body, lead.Status); ok {
			skipTreeForOptionReply = true
			w.log.Info("ai worker: bare option reply detected, skipping decision tree", "lead_id", lead.ID, "status", lead.Status, "message", msg.Body)
		}
	}

	// Decision tree: check if the studio has an active tree that matches this message.
	// If it does, use the tree reply directly and skip the AI call entirely.
	if w.dtSvc != nil && !skipTreeForOptionReply {
		leadStatus := ""
		if lead != nil {
			leadStatus = string(lead.Status)
		}
		treeResult, treeErr := w.dtSvc.TraverseActiveTree(ctx, studioID, msg.Body, leadStatus, conv.CurrentTreeNodeID)
		w.log.Info("decision tree traversal", "studio_id", studioID, "lead_status", leadStatus, "msg", msg.Body, "tree_found", treeResult != nil, "matched", treeResult != nil && treeResult.Matched, "err", treeErr)
		if treeErr != nil {
			w.log.Warn("decision tree traversal failed", "studio_id", studioID, "err", treeErr)
		} else if treeResult != nil && treeResult.Matched {
			// Remember where this conversation is in the flow so the next
			// message is checked against this node's children first — but
			// only for a plain reply ("still chatting"). Every other action
			// hands the conversation off elsewhere (human, the auto-contact
			// state machine, a status change), so reset to root for next time.
			nextNodeID := treeResult.NodeID
			if treeResult.Action != decisiontree.ActionReply {
				nextNodeID = nil
			}
			if err := w.msgRepo.SetConversationTreeNode(ctx, studioID, msg.ConversationID, nextNodeID); err != nil {
				w.log.Warn("failed to update conversation tree stage", "studio_id", studioID, "err", err)
			}

			// Pipeline: change lead status before or alongside reply.
			if treeResult.TargetStatus != "" && lead != nil {
				newStatus := leads.LeadStatus(treeResult.TargetStatus)
				if newStatus.Valid() {
					if err := w.leadsRepo.UpdateStatus(ctx, studioID, lead.ID, newStatus); err != nil {
						w.log.Warn("decision tree status change failed", "studio_id", studioID, "err", err)
					} else {
						w.log.Info("decision tree changed lead status", "studio_id", studioID, "lead_id", lead.ID, "status", treeResult.TargetStatus)
					}
				}
			}

			switch treeResult.Action {
			case decisiontree.ActionReply, decisiontree.ActionChangeStatus:
				// ActionChangeStatus can also carry a reply — handle both the same way.
				reply := treeResult.Reply
				if reply == "" {
					// change_status with no reply: silently update status and stop.
					return nil
				}
				reply = renderTreeReply(reply, lead, studio)
				w.log.Info("decision tree matched, using tree reply", "studio_id", studioID, "node", treeResult.NodeLabel)
				_, err = w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
					StudioID:       studioID,
					ConversationID: msg.ConversationID,
					Body:           reply,
					SourceKind:     SourceAI,
					SourceRef:      "decision_tree",
					ScheduledFor:   time.Now().UTC().Add(replyDelay),
				})
				if err != nil {
					return fmt.Errorf("enqueue tree reply: %w", err)
				}
				w.bus.Publish(ctx, Event{
					Kind:           EvtOutboundJobEnqueued,
					StudioID:       studioID,
					ConversationID: msg.ConversationID,
				})
				return nil
			case decisiontree.ActionEscalate:
				w.log.Info("decision tree matched, escalating to human", "studio_id", studioID, "node", treeResult.NodeLabel)
				if err := w.msgSvc.EscalateAndNotify(ctx, studioID, msg.ConversationID, treeResult.NodeLabel); err != nil {
					w.log.Warn("failed to mark conversation escalated", "studio_id", studioID, "err", err)
				}
				if treeResult.Reply != "" {
					reply := renderTreeReply(treeResult.Reply, lead, studio)
					if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
						StudioID:       studioID,
						ConversationID: msg.ConversationID,
						Body:           reply,
						SourceKind:     SourceAI,
						SourceRef:      "decision_tree",
						ScheduledFor:   time.Now().UTC().Add(replyDelay),
					}); err == nil {
						w.bus.Publish(ctx, Event{
							Kind:           EvtOutboundJobEnqueued,
							StudioID:       studioID,
							ConversationID: msg.ConversationID,
						})
					}
				}
				return nil
			case decisiontree.ActionBookTrial:
				// A lead who's already been a trialist (lifetime, regardless
				// of their current status) never sees the options menu again
				// — go straight to the ineligibility reply instead of
				// offering "1. Book a Trial" to someone who can't.
				if lead != nil && lead.HasBeenTrialist() {
					w.log.Info("decision tree matched book_trial, but lead already a trialist — sending ineligible reply", "studio_id", studioID, "node", treeResult.NodeLabel, "lead", lead.ID)
					firstName := lead.FirstName
					if firstName == "" {
						firstName = lead.Name
					}
					if firstName == "" {
						firstName = conv.ContactDisplayName
					}
					if firstName == "" {
						firstName = "there"
					}
					_, err = w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
						StudioID:       studioID,
						ConversationID: msg.ConversationID,
						Body:           trialIneligibleBody(firstName),
						SourceKind:     SourceAI,
						SourceRef:      "trial_ineligible",
						ScheduledFor:   time.Now().UTC().Add(replyDelay),
					})
					if err != nil {
						return fmt.Errorf("enqueue trial-ineligible reply: %w", err)
					}
					w.bus.Publish(ctx, Event{
						Kind:           EvtOutboundJobEnqueued,
						StudioID:       studioID,
						ConversationID: msg.ConversationID,
					})
					return nil
				}
				// Tree matched a booking node — drive the lead into the options menu.
				w.log.Info("decision tree matched book_trial, triggering booking flow", "studio_id", studioID, "node", treeResult.NodeLabel)
				if lead != nil {
					if err := w.leadsRepo.UpdateAutoContactStage(ctx, studioID, lead.ID, "awaiting_options"); err != nil {
						w.log.Warn("book_trial: failed to set awaiting_options", "lead", lead.ID, "err", err)
					}
				}
				bookBody := treeResult.Reply
				if bookBody == "" {
					bookBody = "Great! Please select an option:\n1. Book a Trial\n2. Become a Member"
				}
				bookBody = renderTreeReply(bookBody, lead, studio)
				_, err = w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
					StudioID:       studioID,
					ConversationID: msg.ConversationID,
					Body:           bookBody,
					SourceKind:     SourceAutomation,
					SourceRef:      fmt.Sprintf("decision_tree:%s", treeResult.NodeLabel),
					ScheduledFor:   time.Now().UTC().Add(replyDelay),
				})
				if err != nil {
					return fmt.Errorf("enqueue book_trial reply: %w", err)
				}
				w.bus.Publish(ctx, Event{
					Kind:           EvtOutboundJobEnqueued,
					StudioID:       studioID,
					ConversationID: msg.ConversationID,
				})
				return nil
			case decisiontree.ActionSendLink:
				// Treat send_link like reply — use the node's reply template.
				reply := treeResult.Reply
				if reply == "" {
					return nil
				}
				reply = renderTreeReply(reply, lead, studio)
				w.log.Info("decision tree matched send_link, sending reply", "studio_id", studioID, "node", treeResult.NodeLabel)
				_, err = w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
					StudioID:       studioID,
					ConversationID: msg.ConversationID,
					Body:           reply,
					SourceKind:     SourceAI,
					SourceRef:      "decision_tree",
					ScheduledFor:   time.Now().UTC().Add(replyDelay),
				})
				if err != nil {
					return fmt.Errorf("enqueue send_link reply: %w", err)
				}
				w.bus.Publish(ctx, Event{
					Kind:           EvtOutboundJobEnqueued,
					StudioID:       studioID,
					ConversationID: msg.ConversationID,
				})
				return nil
			}
		} else if treeResult == nil && w.dtSvc != nil {
			w.log.Debug("decision tree: no active tree found", "studio_id", studioID, "lead_status", func() string {
				if lead != nil {
					return string(lead.Status)
				}
				return ""
			}())
		} else if treeResult != nil && !treeResult.Matched {
			w.log.Debug("decision tree: no node matched", "studio_id", studioID, "message", msg.Body)
		}
	}

	// The AI can't book, cancel, move or check availability for a class, so any such
	// request goes straight to a person (trial booking has its own flow and is excluded).
	// Runs before the booking shortcut below, which would otherwise answer "I booked two
	// classes today..." with the Book a Trial / Become a Member menu.
	if isClassActionRequest(msg.Body) {
		w.log.Info("ai worker: class booking/change request, escalating to a human",
			"studio_id", studioID, "conversation_id", conv.ID)
		w.escalateWithHandoff(ctx, studioID, conv.ID,
			"Customer asked to book or change a class (the AI can't do this)",
			"Thanks for letting us know! I can't book or change classes myself, so I've asked a team member to help you with this. They'll be with you shortly!",
			"class_request_handoff", replyDelay)
		return nil
	}

	// Booking shortcut: when customer says "trail"/"book trial" and the bot isn't mid-flow,
	// jump directly into the automation stage machine instead of using AI.
	if intent == "booking_inquiry" {
		notInActiveFlow := true
		if lead != nil {
			stage := lead.AutoContactStage
			notInActiveFlow = stage == "" || stage == "completed" || stage == "awaiting_interest" || stage == "awaiting_options"
		}
		if notInActiveFlow {
			// Update stage if we have a lead
			if lead != nil {
				if err := w.leadsRepo.UpdateAutoContactStage(ctx, studioID, lead.ID, "awaiting_options"); err != nil {
					w.log.Warn("booking shortcut: failed to set awaiting_options", "lead", lead.ID, "err", err)
				}
			}
			sourceRef := "booking_shortcut"
			if lead != nil {
				sourceRef = fmt.Sprintf("lead:%s:booking_shortcut", lead.ID)
			}
			body := "Great! Please select an option:\n1. Book a Trial\n2. Become a Member"
			if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
				StudioID:       studioID,
				ConversationID: conv.ID,
				Body:           body,
				SourceKind:     SourceAutomation,
				SourceRef:      sourceRef,
				ScheduledFor:   time.Now().UTC().Add(replyDelay),
			}); err != nil {
				return fmt.Errorf("booking shortcut enqueue: %w", err)
			}
			w.bus.Publish(ctx, Event{
				Kind:           EvtOutboundJobEnqueued,
				StudioID:       studioID,
				ConversationID: conv.ID,
			})
			w.log.Info("booking shortcut triggered, skipping AI", "conv", conv.ID)
			return nil
		}
	}

	// Membership shortcut: when the classifier says the customer is ready to
	// buy ("I want to become a member" etc.) and the bot isn't mid-flow,
	// jump into the automation stage machine the same way the booking
	// shortcut above does — otherwise the LLM free-generates its own
	// membership pitch, including inventing a fake payment link and asking
	// for a "transaction ID" (a flow that doesn't exist anywhere in this
	// codebase), instead of sending the real Stripe checkout link. intent
	// only gets set to "ready_to_buy" when Gemini is configured — looksReadyToBuy
	// is the provider-agnostic fallback for everyone else (real incident: a
	// studio without Gemini got a reply promising "I'll pass this over to a
	// team member who can securely share the purchase link" for "Send me a
	// link to buy?" — nothing was ever actually escalated).
	if intent == "ready_to_buy" || looksReadyToBuy(lowerBody) {
		notInActiveFlow := true
		if lead != nil {
			stage := lead.AutoContactStage
			notInActiveFlow = stage == "" || stage == "completed" || stage == "awaiting_interest" || stage == "awaiting_options"
		}
		if notInActiveFlow {
			if lead != nil {
				if err := w.leadsRepo.UpdateAutoContactStage(ctx, studioID, lead.ID, "awaiting_options"); err != nil {
					w.log.Warn("membership shortcut: failed to set awaiting_options", "lead", lead.ID, "err", err)
				}
			}
			sourceRef := "membership_shortcut"
			if lead != nil {
				sourceRef = fmt.Sprintf("lead:%s:membership_shortcut", lead.ID)
			}
			body := "Great! Please select an option:\n1. Book a Trial\n2. Become a Member"
			if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
				StudioID:       studioID,
				ConversationID: conv.ID,
				Body:           body,
				SourceKind:     SourceAutomation,
				SourceRef:      sourceRef,
				ScheduledFor:   time.Now().UTC().Add(replyDelay),
			}); err != nil {
				return fmt.Errorf("membership shortcut enqueue: %w", err)
			}
			w.bus.Publish(ctx, Event{
				Kind:           EvtOutboundJobEnqueued,
				StudioID:       studioID,
				ConversationID: conv.ID,
			})
			w.log.Info("membership shortcut triggered, skipping AI", "conv", conv.ID)
			return nil
		}
	}

	// A bare "1"/"2" reply (or clear trial/member keyword) to our own menu is handled
	// deterministically, not left for the LLM — and lead-independent on purpose, since
	// detectOptionChoice/updateLeadStatus below only run for a lead-linked conversation.
	// SendTrialPaymentLink handles a nil lead safely (escalating if it can't process the
	// trial, e.g. Stripe not configured).
	if lastOutboundSentOurMenu(history) {
		wantsTrial, wantsMember := parseMenuChoice(msg.Body)
		if wantsTrial && !wantsMember {
			firstName := conv.ContactDisplayName
			if lead != nil && lead.FirstName != "" {
				firstName = lead.FirstName
			} else if lead != nil && lead.Name != "" {
				firstName = lead.Name
			}
			if firstName == "" {
				firstName = "there"
			}
			if lead != nil {
				if err := w.leadsRepo.UpdateAutoContactStage(ctx, studioID, lead.ID, "awaiting_options"); err != nil {
					w.log.Warn("menu-choice shortcut: failed to set awaiting_options", "lead", lead.ID, "err", err)
				}
			}
			if _, err := w.msgSvc.SendTrialPaymentLink(ctx, studioID, conv.ID, conv.LeadID, firstName); err != nil {
				w.log.Error("menu-choice shortcut: failed to send trial payment link", "err", err, "conv", conv.ID)
			} else {
				w.bus.Publish(ctx, Event{Kind: EvtOutboundJobEnqueued, StudioID: studioID, ConversationID: conv.ID})
				w.log.Info("menu-choice shortcut triggered (trial)", "conv", conv.ID, "lead_linked", lead != nil)
			}
			return nil
		}
		// No leadless membership-checkout flow exists (checkout links are built
		// per-lead), so escalate instead of guessing — same principle as the trial
		// case above, just without a link to offer.
		if wantsMember && !wantsTrial && lead == nil {
			w.escalateWithHandoff(ctx, studioID, conv.ID,
				"Customer wants to become a member — no lead on this conversation to build a checkout link for",
				"That's great to hear! Let me get one of our team members to help you sign up — they'll be with you shortly.",
				"menu_choice_no_lead_member_handoff", replyDelay)
			w.log.Info("menu-choice shortcut triggered (member, no lead -> escalated)", "conv", conv.ID)
			return nil
		}
	}

	// "Yes to trial" shortcut: skip AI and send the payment link directly
	// when either (a) the customer explicitly asks to buy/pay — but not when
	// that's phrased as a question ("how do I pay?", "what's the payment
	// link?" are asking, not confirming) — or (b) they give a bare
	// affirmation ("yes"/"keen") replying to a bot message that had just
	// offered a trial.
	//
	// Deliberately an EXACT match only, not a prefix/contains check: "yes "
	// used to match anything starting with "yes" (e.g. "Yes I would like
	// directions"), hijacking an unrelated question into a trial-confirm
	// shortcut before the AI ever saw it. A real affirmation is short —
	// anything with more content after it is a different topic and should
	// go through the AI's normal reasoning instead.
	if lead != nil {
		lowerMsg := strings.ToLower(strings.TrimSpace(msg.Body))
		explicitPurchaseIntent := isExplicitPurchaseIntent(lowerMsg)
		isAffirmative := lowerMsg == "yes" || lowerMsg == "yeah" || lowerMsg == "sure" ||
			lowerMsg == "ok" || lowerMsg == "okay" || lowerMsg == "yep" || lowerMsg == "yup" ||
			lowerMsg == "y" || lowerMsg == "keen" || lowerMsg == "im keen" || lowerMsg == "i'm keen" ||
			lowerMsg == "interested" || lowerMsg == "i'm interested" || lowerMsg == "im interested"

		if explicitPurchaseIntent {
			firstName := lead.FirstName
			if firstName == "" {
				firstName = lead.Name
			}
			if _, err := w.msgSvc.SendTrialPaymentLink(ctx, studioID, conv.ID, conv.LeadID, firstName); err != nil {
				w.log.Error("buy-trial: failed to send payment link", "err", err, "conv", conv.ID)
			} else {
				w.bus.Publish(ctx, Event{Kind: EvtOutboundJobEnqueued, StudioID: studioID, ConversationID: conv.ID})
				w.log.Info("buy-trial shortcut triggered (explicit purchase intent)", "conv", conv.ID)
			}
			return nil
		}

		if isAffirmative {
			// Check the last few outbound messages (not just the single most
			// recent one) — back-to-back bot replies can otherwise push a
			// genuine trial offer just out of view.
			checked := 0
			for i := len(history) - 1; i >= 0 && checked < 3; i-- {
				m := history[i]
				if m.Direction != DirectionOutbound {
					continue
				}
				checked++
				lastBot := strings.ToLower(m.Body)
				offeredTrial := strings.Contains(lastBot, "trial") &&
					(strings.Contains(lastBot, "would you like") ||
						strings.Contains(lastBot, "book a trial") ||
						strings.Contains(lastBot, "sign up for a trial") ||
						strings.Contains(lastBot, "try") ||
						strings.Contains(lastBot, "experience"))
				if offeredTrial {
					firstName := lead.FirstName
					if firstName == "" {
						firstName = lead.Name
					}
					if _, err := w.msgSvc.SendTrialPaymentLink(ctx, studioID, conv.ID, conv.LeadID, firstName); err != nil {
						w.log.Error("yes-to-trial: failed to send payment link", "err", err, "conv", conv.ID)
					} else {
						w.bus.Publish(ctx, Event{Kind: EvtOutboundJobEnqueued, StudioID: studioID, ConversationID: conv.ID})
						w.log.Info("yes-to-trial shortcut triggered", "conv", conv.ID)
					}
					return nil
				}
			}
		}
	}

	// "Yes to upgrade" shortcut: an existing member replies affirmatively to
	// the AI's own offer (added above, in the pricing_question/member response
	// strategy) to change or upgrade their plan. Skip AI and send the plan list
	// directly, mirroring the yes-to-trial shortcut above.
	if lead != nil && lead.Status == leads.StatusMember {
		lowerMsg := strings.ToLower(strings.TrimSpace(msg.Body))
		isAffirmative := lowerMsg == "yes" || lowerMsg == "yeah" || lowerMsg == "sure" ||
			lowerMsg == "ok" || lowerMsg == "okay" || lowerMsg == "yep" || lowerMsg == "yup" ||
			lowerMsg == "y" || strings.HasPrefix(lowerMsg, "yes ") || strings.HasPrefix(lowerMsg, "sure ")
		if isAffirmative {
			for i := len(history) - 1; i >= 0; i-- {
				m := history[i]
				if m.Direction == DirectionOutbound {
					lastBot := strings.ToLower(m.Body)
					offeredChange := (strings.Contains(lastBot, "change") || strings.Contains(lastBot, "upgrade") || strings.Contains(lastBot, "switch")) &&
						(strings.Contains(lastBot, "plan") || strings.Contains(lastBot, "membership"))
					if offeredChange {
						plans, err := w.msgRepo.ListActivePlans(ctx, studioID)
						if err != nil || len(plans) == 0 {
							w.log.Warn("yes-to-upgrade: no active plans to offer", "conv", conv.ID, "err", err)
							return nil
						}
						if err := w.leadsRepo.UpdateAutoContactStage(ctx, studioID, lead.ID, "awaiting_plan_change_selection"); err != nil {
							w.log.Warn("yes-to-upgrade: failed to set stage", "lead", lead.ID, "err", err)
						}
						body := formatPlanReprompt("Sure! Which plan would you like to switch to?", plans)
						if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
							StudioID:       studioID,
							ConversationID: conv.ID,
							Body:           body,
							SourceKind:     SourceAutomation,
							SourceRef:      fmt.Sprintf("lead:%s:plan_change_options", lead.ID),
							ScheduledFor:   time.Now().UTC().Add(replyDelay),
						}); err != nil {
							w.log.Error("yes-to-upgrade: failed to enqueue plan list", "err", err, "conv", conv.ID)
						} else {
							w.bus.Publish(ctx, Event{Kind: EvtOutboundJobEnqueued, StudioID: studioID, ConversationID: conv.ID})
							w.log.Info("yes-to-upgrade shortcut triggered", "conv", conv.ID)
						}
						return nil
					}
					break
				}
			}
		}
	}

	// Low-confidence pricing/general/class questions escalate instead of guessing. Small
	// talk and booking-flow messages are exempt — they don't depend on KB grounding.
	lowConfidenceFactual := intent == "pricing_question" || intent == "general_question"
	lowConfidenceClass := retrievalRan && isClassRelatedQuery(msg.Body)
	if !kbConfident && (lowConfidenceFactual || lowConfidenceClass) {
		w.log.Info("ai worker: low KB confidence on a factual/class question, escalating instead of guessing",
			"studio_id", studioID, "conversation_id", conv.ID, "intent", intent, "class_related", lowConfidenceClass)
		w.escalateKnowledgeGap(ctx, studioID, conv, lead, msg.Body,
			"AI uncertain — insufficient knowledge base match", "low_confidence_handoff", replyDelay)
		return nil
	}

	// Provider-agnostic hard gate, checked before the prompt is built: if nothing in the
	// knowledge base covers the question, escalate regardless of which model would answer.
	if !kbCoversQuestion(availableKnowledgeText(kbChunks, studio), msg.Body) {
		w.log.Info("ai worker: knowledge base has no coverage for this question, escalating before calling any model",
			"studio_id", studioID, "conversation_id", conv.ID)
		w.escalateKnowledgeGap(ctx, studioID, conv, lead, msg.Body,
			"No knowledge base coverage for this question", "no_kb_coverage_handoff", replyDelay)
		return nil
	}

	// Generate AI response with context
	aiContextSummary, err := w.msgRepo.GetConversationAISummary(ctx, studioID, conv.ID)
	if err != nil {
		w.log.Warn("fetch conversation ai summary failed", "err", err)
		aiContextSummary = ""
	}
	prompt := w.buildPrompt(ctx, history, styleExamples, conv, lead, studio, plans, offers, sentiment, keywords, kbChunks, intent, kbConfident, aiContextSummary)

	// Shadow mode (ai-reply-flow-llm-design task 9.2): when enabled for
	// this studio, everything from here on (structured call, tools,
	// guards) still runs for real against this real inbound message, but
	// the reply is never enqueued and no escalation/alert fires — only an
	// ai_turns row is written, with outcome "shadow", so the new
	// pipeline's real behavior against real traffic can be reviewed
	// before this studio's customers are switched onto it. Does not
	// affect the hard knowledge-gap gates above (kbCoversQuestion etc.) —
	// those are pre-existing behavior this change doesn't alter, not part
	// of what shadow mode is validating.
	shadowMode, shadowErr := w.msgRepo.aiShadowMode(ctx, studioID)
	if shadowErr != nil {
		w.log.Warn("read ai shadow mode failed, defaulting to live", "studio_id", studioID, "err", shadowErr)
		shadowMode = false
	}

	var resp string
	var sourceRef string
	pipelineStart := time.Now()

	{
		// One structured call per debounced batch (ai-reply-flow-llm-design
		// task 3.2), replacing the old plain-text Complete call — the
		// provider waterfall (order, per-provider settings, time budget)
		// still lives in ai_feature_routes under feature "customer_reply",
		// applied via llmGateway, unchanged by this.
		structured, structuredRef, err := llmgateway.CompleteStructured[StructuredReply](
			ctx, w.llmGateway, llmgateway.FeatureCustomerReply, studioID, prompt,
			StructuredReplySchemaName, StructuredReplyJSONSchema,
		)
		guardsPassed := true
		handoffRequested := false
		var structuredForTurn any
		var toolCallsForTurn any
		if err != nil {
			w.log.Error("llm gateway: structured customer reply failed", "studio_id", studioID, "err", err)
		} else {
			sourceRef = structuredRef
			structuredForTurn = structured

			// Tool calls (ai-reply-flow-llm-design task 7.2), bounded to
			// maxToolCallRounds total LLM calls (this first one plus at
			// most one follow-up) — a model that asks for tools again on
			// the follow-up is not honored a 3rd round; its reply is used
			// as final regardless. In shadow mode, a side-effecting call
			// (create_booking) is recorded as requested but not actually
			// executed — see executeToolCalls' shadow parameter.
			if len(structured.ToolCalls) > 0 {
				results := w.executeToolCalls(ctx, studioID, conv.ID, lead, structured.ToolCalls, shadowMode)
				toolCallsForTurn = results
				if maxToolCallRounds > 1 {
					followupPrompt := appendToolResultsToPrompt(prompt, structured.ToolCalls, results)
					followup, followupRef, followupErr := llmgateway.CompleteStructured[StructuredReply](
						ctx, w.llmGateway, llmgateway.FeatureCustomerReply, studioID, followupPrompt,
						StructuredReplySchemaName, StructuredReplyJSONSchema,
					)
					if followupErr == nil {
						structured = followup
						sourceRef = followupRef
						structuredForTurn = structured
					} else {
						w.log.Warn("tool-call follow-up structured call failed, using pre-tool reply", "studio_id", studioID, "err", followupErr)
					}
				}
			}

			w.log.Info("structured reply",
				"studio_id", studioID, "conversation_id", conv.ID,
				"action", structured.Action, "language", structured.Language,
				"intent", structured.Intent, "sentiment", structured.Sentiment,
				"facts_used", structured.FactsUsed, "tool_calls", len(structured.ToolCalls),
				"shadow", shadowMode)

			// The structured call's own "handoff" action (ai-reply-flow-llm-
			// design: replaces the old pre-reply ClassifyIntent-based
			// complaint check, now that classification happens inline with
			// generation rather than before it) — a complaint, negative
			// feedback, bad-review threat, or other sensitive request the
			// free keyword checks above didn't already catch. No guard
			// check or send; this goes to a person instead.
			if structured.Action == "handoff" {
				handoffRequested = true
			} else {
				// Guard check + bounded repair (ai-reply-flow-llm-design task 4.5),
				// before this reply is cached or sent.
				pack := buildGuardDataPack(plans, offers, structured.Language, knownCustomerFieldsFromLead(lead))
				repaired, ok := w.checkAndRepair(ctx, studioID, structured, pack)
				if ok {
					resp = repaired
				} else {
					guardsPassed = false
				}
			}
		}

		latencyMS := int(time.Since(pipelineStart).Milliseconds())
		guardOutcome := "passed"
		switch {
		case handoffRequested:
			guardOutcome = "n/a"
		case !guardsPassed:
			guardOutcome = "failed"
		}
		turnOutcome := "sent"
		switch {
		case handoffRequested:
			turnOutcome = "handoff"
		case !guardsPassed:
			turnOutcome = "guard_failed"
		case resp == "":
			turnOutcome = "all_providers_failed"
		case shadowMode:
			turnOutcome = "shadow"
		}
		if turnErr := w.msgRepo.InsertAITurn(ctx, AITurn{
			ConversationID:   conv.ID,
			StudioID:         studioID,
			PipelineVersion:  aiPipelineVersion,
			StructuredOutput: structuredForTurn,
			GuardOutcome:     guardOutcome,
			ToolCalls:        toolCallsForTurn,
			LatencyMS:        &latencyMS,
			Outcome:          turnOutcome,
		}); turnErr != nil {
			w.log.Warn("insert ai turn failed", "studio_id", studioID, "conversation_id", conv.ID, "err", turnErr)
		}

		if shadowMode {
			w.log.Info("ai worker: shadow mode — pipeline ran and was recorded, reply not sent",
				"studio_id", studioID, "conversation_id", conv.ID, "outcome", turnOutcome)
			return nil
		}

		if handoffRequested {
			w.log.Info("ai worker: structured reply requested human handoff", "studio_id", studioID, "conversation_id", conv.ID)
			w.escalateWithHandoff(ctx, studioID, conv.ID,
				"AI determined this conversation needs a human (complaint, negative feedback, or sensitive request)",
				complaintHandoffBody, "ai_requested_handoff", replyDelay)
			return nil
		}

		if !guardsPassed {
			w.log.Warn("ai reply: failed guard checks even after repair, escalating to staff", "studio_id", studioID, "conversation_id", conv.ID)
			w.msgRepo.RecordAlert(ctx, "guard_check_failed", map[string]any{
				"conversation_id": conv.ID,
				"message_id":      messageID,
			}, studioID)
			w.escalateWithHandoff(ctx, studioID, conv.ID,
				"AI reply failed a safety guard check and could not be repaired",
				guardFailedHandoffBody, "guard_failed_handoff", replyDelay)
			return nil
		}

		if resp == "" {
			w.log.Warn("ai reply: all providers failed or not configured, escalating to staff", "studio_id", studioID, "conversation_id", conv.ID)
			w.msgRepo.RecordAlert(ctx, "all_providers_failed", map[string]any{
				"conversation_id": conv.ID,
				"message_id":      messageID,
			}, studioID)
			w.escalateWithHandoff(ctx, studioID, conv.ID,
				"All AI providers failed to generate a reply",
				allProvidersFailedHandoffBody, "all_providers_failed_handoff", replyDelay)
			return nil
		}
	}

	// Second, provider-agnostic safety net for studios without Gemini (where the
	// low-confidence check above never runs): escalate if the reply itself reads as
	// "I don't know" rather than sending it.
	if looksLikeAIDoesNotKnow(resp) {
		w.log.Info("ai worker: reply indicates the AI doesn't know the answer, escalating instead of sending it",
			"studio_id", studioID, "conversation_id", conv.ID, "source", sourceRef)
		w.escalateKnowledgeGap(ctx, studioID, conv, lead, msg.Body,
			"AI's own reply indicated it doesn't know the answer", "ai_uncertain_handoff", replyDelay)
		return nil
	}

	// Post-process: strip motivation questions when customer clearly wants to book.
	// Groq's smaller models ignore the prompt instruction reliably, so we enforce it here.
	if intent == "booking_inquiry" || historyAlreadyAskedAboutGoals(history) {
		resp = stripMotivationQuestions(resp)
	}

	// Never open with a time-of-day greeting — see stripTimeGreeting's doc comment.
	resp = stripTimeGreeting(resp)

	// Don't re-greet by name on every turn of an active exchange — the prompt
	// already instructs this (see conversationResumedAfterGap's use in
	// buildPrompt), but models don't follow it reliably, so it's enforced
	// here too, same reasoning as stripTimeGreeting above.
	if !conversationResumedAfterGap(history, 2*time.Hour) {
		resp = stripNameGreeting(resp)
	}

	if guarded := guardAgainstPlaceholderArtifact(resp); guarded != resp {
		w.log.Warn("ai response contained an unfilled template placeholder — replaced with fallback", "message_id", msg.ID, "studio_id", studioID, "original_response", resp)
		resp = guarded
	}

	w.log.Info("ai response generated", "message_id", msg.ID, "response_len", len(resp), "channel", channel.Kind, "model", sourceRef)

	// Enqueue outbound reply. Delayed slightly so the reply doesn't feel
	// instant/robotic to the recipient.
	_, err = w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: msg.ConversationID,
		Body:           resp,
		SourceKind:     SourceAI,
		SourceRef:      sourceRef,
		ScheduledFor:   time.Now().UTC().Add(20 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("enqueue ai outbound: %w", err)
	}

	w.bus.Publish(ctx, Event{
		Kind:           EvtOutboundJobEnqueued,
		StudioID:       studioID,
		ConversationID: msg.ConversationID,
	})

	// Update lead status based on sentiment or explicit choices
	// Update for ALL leads, not just new ones - helps track progression through pipeline
	if lead != nil {
		w.updateLeadStatus(ctx, studioID, lead, msg.Body, sentiment, confidence)
	}

	return nil
}

// availableKnowledgeText is exactly what gets shown to the model as "KNOWLEDGE BASE:"
// — reranked chunks if retrieval ran and found any, otherwise the studio's whole raw
// text and document uploads (the no-retrieval fallback). Shared by buildPrompt and
// kbCoversQuestion's pre-check so the two can never drift apart: whatever the model
// is actually shown is exactly what the pre-check judges coverage against.
func availableKnowledgeText(kbChunks []string, studio *studios.Studio) string {
	if len(kbChunks) > 0 {
		return strings.Join(kbChunks, "\n\n")
	}
	if studio == nil {
		return ""
	}
	kbText := studio.KnowledgeBase
	for _, f := range studio.KnowledgeBaseFiles {
		if f.Text != "" {
			kbText += fmt.Sprintf("\n\nDocument (%s):\n%s", f.Name, f.Text)
		}
	}
	return kbText
}

// renderTreeReply substitutes a decision-tree node's documented template
// variables — {{lead_name}}, {{lead_first_name}}, {{lead_status}},
// {{studio_name}} (the exact set the TreeEditor UI tells admins they can
// use, see its reply-template field help text) — into its ReplyTemplate.
// Shared by every action branch below (reply/change_status, escalate,
// book_trial, send_link): previously each branch inlined its own
// subset of these substitutions, and escalate/book_trial/send_link didn't
// substitute {{studio_name}} at all — a template using it on one of those
// node types would send the literal, unfilled "{{studio_name}}" to a real
// customer. One shared helper means every node type gets every documented
// variable, with no risk of a future branch forgetting one again.
func renderTreeReply(reply string, lead *leads.Lead, studio *studios.Studio) string {
	if lead != nil {
		reply = strings.ReplaceAll(reply, "{{lead_name}}", lead.Name)
		reply = strings.ReplaceAll(reply, "{{lead_first_name}}", lead.FirstName)
		reply = strings.ReplaceAll(reply, "{{lead_status}}", string(lead.Status))
	}
	if studio != nil {
		reply = strings.ReplaceAll(reply, "{{studio_name}}", studio.Name)
	}
	// Decision tree templates are admin-authored, not AI-generated, but the
	// same mistake happens here: an admin types "[Your Name]" (or similar)
	// expecting it to be filled in, not realizing it isn't one of the
	// documented {{...}} variables the TreeEditor UI actually supports —
	// confirmed happening in production. Unlike an AI-generated reply
	// (guardAgainstPlaceholderArtifact swaps the whole message, since the
	// model's other content can't be trusted either), this is otherwise
	// real, intentional content the admin wrote — swap just the
	// placeholder for the studio name rather than discarding their whole
	// message every time this template fires. Not a perfect guess for
	// every kind of placeholder (e.g. a missing address), but strictly
	// better than sending raw "[Your Name]" to a real customer, and the
	// warning log below gives an admin something to find and fix properly.
	if studio != nil && unfilledPlaceholder.MatchString(reply) {
		slog.Warn("decision tree reply template contains an unfilled placeholder", "studio_id", studio.ID, "reply", reply)
		reply = unfilledPlaceholder.ReplaceAllString(reply, studio.Name)
	}
	return reply
}

// buildPrompt composes the full prompt for a customer reply. It never asks the model to
// open with a time-of-day greeting ("Good morning" etc.); stripTimeGreeting removes one if
// the model adds it anyway.
func (w *AIWorker) buildPrompt(ctx context.Context, history []Message, styleExamples []StyleExample, conv *Conversation, lead *leads.Lead, studio *studios.Studio, plans []Plan, offers []Offer, sentiment int, keywords []string, kbChunks []string, intent string, kbConfident bool, aiContextSummary string) string {
	var sb strings.Builder

	// ── System role ──────────────────────────────────────────────────────────
	// (ai-reply-flow-llm-design task 2.4: this static text now lives in
	// internal/messaging/prompts/system_rules.tmpl, reviewable as a plain
	// diff — see that package's doc comment for which parts of this
	// function were and weren't converted to template files.)
	sb.WriteString(prompts.MustRender(prompts.SystemRules, prompts.SystemRulesData{Tone: prompts.ToneSnippet(studio.ReplyTone)}))

	now := time.Now().UTC()

	// Resolve the studio's local timezone once — used both for "today's date"
	// below and the greeting hour further down. Falls back to UTC if the
	// studio has no AvailabilityTimezone set or it fails to load.
	loc := time.UTC
	if studio != nil && studio.AvailabilityTimezone != "" {
		if l, err := time.LoadLocation(studio.AvailabilityTimezone); err == nil {
			loc = l
		}
	}
	localNow := now.In(loc)

	// Ground the model in the actual current date — without this it has no
	// way to know what day "today" is, so it can't correctly answer
	// schedule questions ("which class is today?") against a knowledge-base
	// timetable that's organized by day of week.
	sb.WriteString(prompts.MustRender(prompts.TodayGrounding, prompts.TodayGroundingData{
		Weekday:  localNow.Format("Monday"),
		Date:     localNow.Format("January 2, 2006"),
		Timezone: loc.String(),
	}))

	// Deterministic program-session lookup — real date math done here in
	// code plus an exact database row, not the LLM guessing among many
	// near-identical weekly entries via semantic search (which reliably
	// picked the wrong week — the "Week 4 Tuesday" investigation this was
	// built from). Only fires when a studio has both set program_start_date
	// AND has a parsed program-schedule document on file (see
	// ParseProgramSchedule); does nothing otherwise, and the week-arithmetic
	// fallback instruction below still covers that case.
	if studio != nil && studio.ProgramStartDate != nil {
		// Extract the stored date's own Y/M/D (not the studio timezone's
		// reinterpretation of it — program_start_date is a plain calendar
		// date with no time-of-day, so converting it through .In(loc) first
		// could shift it a day in either direction depending on the
		// timezone offset) and re-anchor at local midnight for the
		// elapsed-days calculation below.
		sy, smo, sd := studio.ProgramStartDate.Date()
		startDay := time.Date(sy, smo, sd, 0, 0, 0, 0, loc)
		today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
		elapsedDays := int(today.Sub(startDay).Hours() / 24)
		if elapsedDays >= 0 {
			weekNumber := elapsedDays/7 + 1
			dayOfWeek := int(localNow.Weekday())
			if dayOfWeek == 0 {
				dayOfWeek = 7 // ISO: Monday=1..Sunday=7, time.Weekday() has Sunday=0
			}
			if session, err := w.studiosRepo.GetProgramSession(ctx, studio.ID, weekNumber, dayOfWeek); err == nil && session != nil {
				sb.WriteString(fmt.Sprintf(
					"TODAY'S VERIFIED PROGRAM SESSION (Week %d, %s) — looked up directly from the schedule, not a guess. For ANY question about today's session, program day, or current week, use ONLY this — ignore any other week's session mentioned elsewhere in the knowledge base:\nSession: %s\nProgression & changes: %s\n%s\n\n",
					session.WeekNumber, session.DayName, session.SessionName, session.Progression, session.KeyFocus))
			}
		}
	}

	// Grounding "today" as a fact (above) is enough for day-of-week lookups
	// ("which class is today"), but a question like "which WEEK of the
	// program is this" needs actual arithmetic — counting elapsed weeks from
	// a program's stated start date to today — which the model is prone to
	// getting wrong when it does that math silently/informally. This forces
	// it to show the arithmetic instead of pattern-matching to a plausible-
	// sounding week number.
	sb.WriteString(prompts.MustRender(prompts.WeekArithmetic, nil))

	// Schedules/timetables read far better as a table than as prose — do
	// this for every channel that asks. On channels without table rendering
	// (WhatsApp/SMS), the markdown pipe syntax still shows up as plain text,
	// but keeps day/session/time visually grouped by line, which is more
	// scannable than a paragraph even unrendered.
	sb.WriteString(prompts.MustRender(prompts.ScheduleTable, nil))

	// No time-of-day greeting, ever — go straight to the answer.
	sb.WriteString(prompts.MustRender(prompts.NoGreetingOpening, nil))
	// Name-greeting ("Hi Puneeth!") on every single turn of an active
	// back-and-forth reads as robotic and repetitive — only resume with one
	// when the customer has actually come back after a real pause.
	if conversationResumedAfterGap(history, 2*time.Hour) {
		sb.WriteString("The customer is resuming this conversation after a pause of more than 2 hours, so it's fine to greet them by name again. ")
	} else {
		sb.WriteString("This is an active, ongoing exchange (no real pause since the last message) — do NOT greet the customer by name again (no \"Hi [name]!\"); jump straight into the answer, same as you would mid-conversation with a person standing in front of you. ")
	}
	sb.WriteString("\n\n")

	// ── Knowledge base ───────────────────────────────────────────────────────
	kbText := availableKnowledgeText(kbChunks, studio)

	if kbText != "" {
		sb.WriteString("KNOWLEDGE BASE:\n\"\"\"\n")
		sb.WriteString(kbText)
		sb.WriteString("\n\"\"\"\n")
		sb.WriteString("Answer ONLY from the knowledge base above and the plans below. Reply to exactly what the customer asked and nothing else. Never invent a specific fact (a day, time, price, date, class, instructor or policy) that isn't stated above, and never fill a gap with something plausible. If the knowledge base doesn't clearly answer the question — or it is about a different topic than the one asked — do not guess: say a team member will help with that, and keep it to one short sentence. Never write a placeholder like [Insert Address Here] or [Your Name] for information you don't have — that's a template artifact, not an answer; say a team member will help with that instead, same as any other missing fact. If you genuinely can't tell what the customer is asking, ask ONE short clarifying question instead of answering something else. ")
		sb.WriteString("You can see your own earlier replies in RECENT CONVERSATION below — do NOT restate pricing, promotions, or programme details you've already told the customer in this conversation; assume they remember it and only repeat something if they explicitly ask again. Keep replies short and move the conversation forward instead of re-explaining what's already covered.\n\n")
	}

	// ── Staff corrections ───────────────────────────────────────────────────
	// Feedback a staff member gave on a specific past reply (from the Test
	// Chat drawer, or eventually live) — see ai_feedback_corrections /
	// AIFeedbackCorrection. Deliberately placed AFTER the knowledge base and
	// told to take priority over it: a correction exists specifically
	// because the KB-grounded answer was wrong, so it must win when the two
	// conflict, not get silently overridden back to the same mistake.
	if studio != nil {
		if corrections, err := w.studiosRepo.ListAIFeedbackCorrections(ctx, studio.ID, 20); err == nil && len(corrections) > 0 {
			sb.WriteString("STAFF CORRECTIONS — a team member reviewed past AI replies and flagged these as wrong. These take priority over the knowledge base above if they ever conflict. Apply the spirit of each correction to any similar question now, not just an exact repeat of the original question:\n")
			for _, c := range corrections {
				fmt.Fprintf(&sb, "- Question like: %q — wrong answer given: %q — correct answer: %q\n", c.Question, c.OriginalReply, c.Correction)
			}
			sb.WriteString("\n")
		}
	}

	// ── Classes ──────────────────────────────────────────────────────────────
	// This assistant has no booking system. It must never claim to have booked, cancelled,
	// moved or confirmed anything — the real conversations showed it saying "locking you in
	// for Friday" when nothing was booked.
	sb.WriteString("You CANNOT book, cancel, reschedule or check live availability for classes, and you must NEVER say or imply that you have (no \"locking you in\", \"booked\", \"confirmed\", \"I've changed it\"). Any request to do those, or a class question the knowledge base doesn't answer, belongs to a team member.\n\n")

	// ── Plans ────────────────────────────────────────────────────────────────
	if len(plans) > 0 {
		sb.WriteString("MEMBERSHIP PLANS (SGD):\n")
		for _, p := range plans {
			line := fmt.Sprintf("  • %s — S$%.2f/%s | %s",
				p.PlanName, float64(p.PriceSGD)/100.0, p.BillingCycle, strings.Join(p.Features, ", "))
			// payment_link (ai-reply-flow-llm-design task 2.5) — handed
			// directly so the model never constructs/guesses a payment URL.
			if p.PaymentLink != "" {
				line += " | Payment link: " + p.PaymentLink
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// ── Offers (ai-reply-flow-llm-design task 2.5) ──────────────────────────
	// Only ACTIVE offers ever reach this prompt (offers is already filtered
	// by ListActiveOffers at the call site) — an inactive/expired offer
	// must never be mentioned to a customer.
	if len(offers) > 0 {
		sb.WriteString("ACTIVE OFFERS (SGD) — use ONLY these, never mention an offer not listed here:\n")
		for _, o := range offers {
			line := fmt.Sprintf("  • %s — S$%.2f", o.Name, float64(o.PriceSGD)/100.0)
			if o.Description != "" {
				line += " | " + o.Description
			}
			if o.Terms != "" {
				line += " | Terms: " + o.Terms
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// ── Lead context ─────────────────────────────────────────────────────────
	if lead != nil {
		if lead.Status == leads.StatusMember && lead.FitnessPlan != "" {
			sb.WriteString(fmt.Sprintf("LEAD: %s | status: %s | CURRENT PLAN: %s at S$%.2f/month (this is their actual active membership — state these exact details if they ask what their plan is)\n",
				lead.Name, lead.Status, lead.FitnessPlan, lead.MonthlyFee))
		} else {
			sb.WriteString(fmt.Sprintf("LEAD: %s | plan interest: %s | status: %s\n", lead.Name, lead.FitnessPlan, lead.Status))
		}
		// Bot automation owns the numbered-choice flow while it is active.
		// AI answers questions freely but must not inject its own options during
		// bot-managed stages — the bot will present the right choices at the right time.
		botOwnsFlow := lead.AutoContactStage != "" && lead.AutoContactStage != "completed"

		switch lead.Status {
		case leads.StatusNew, leads.StatusContacted:
			if !botOwnsFlow {
				// No active bot flow — AI may append booking options, but only if the
				// customer hasn't already seen them and is asking about booking.
				optionsAlreadySent := false
				for _, m := range history {
					if m.Direction == DirectionOutbound &&
						(strings.Contains(m.Body, "Book a Trial") || strings.Contains(m.Body, "Become a Member")) {
						optionsAlreadySent = true
						break
					}
				}
				customerAsksAboutBooking := false
				// A specific unrelated question (hours, location, pricing details,
				// etc.) shouldn't get the booking menu tacked onto its answer just
				// because this is the lead's first message — only a booking-related
				// message or a bare/generic opener (e.g. "Hi") should trigger it.
				customerAskedUnrelatedQuestion := false
				if len(history) > 0 {
					lastMsg := history[len(history)-1]
					if lastMsg.Direction == DirectionInbound {
						lt := strings.ToLower(strings.TrimSpace(lastMsg.Body))
						customerAsksAboutBooking = strings.Contains(lt, "book") ||
							strings.Contains(lt, "trial") ||
							strings.Contains(lt, "trail") || // common typo for "trial"
							strings.Contains(lt, "join") ||
							strings.Contains(lt, "member") ||
							strings.Contains(lt, "sign up") ||
							strings.Contains(lt, "enroll") ||
							strings.Contains(lt, "register")
						isSpecificQuestion := strings.Contains(lt, "?") ||
							strings.HasPrefix(lt, "what") || strings.HasPrefix(lt, "how") ||
							strings.HasPrefix(lt, "when") || strings.HasPrefix(lt, "where") ||
							strings.HasPrefix(lt, "why") || strings.HasPrefix(lt, "is ") ||
							strings.HasPrefix(lt, "are ") || strings.HasPrefix(lt, "do ") ||
							strings.HasPrefix(lt, "can ")
						customerAskedUnrelatedQuestion = isSpecificQuestion && !customerAsksAboutBooking
					}
				}
				if customerAsksAboutBooking || (!optionsAlreadySent && !customerAskedUnrelatedQuestion) {
					sb.WriteString("END your reply with these exact options:\n  1. Book a Trial\n  2. Become a Member\n\n")
				}
			}
		case leads.StatusTrialBooked:
			if !botOwnsFlow {
				// Only show follow-up options if not already shown recently.
				optionsAlreadySent := false
				for _, m := range history {
					if m.Direction == DirectionOutbound &&
						strings.Contains(m.Body, "ready to become a member") {
						optionsAlreadySent = true
						break
					}
				}
				if !optionsAlreadySent {
					sb.WriteString("END your reply with these exact options:\n  1. Yes, I am ready to become a member!\n  2. Not right now\n\n")
				}
			}
		}
	}

	// ── Intent-aware instructions ─────────────────────────────────────────────
	sb.WriteString("RESPONSE STRATEGY:\n")
	switch intent {
	case "pricing_question":
		if lead != nil && lead.Status == leads.StatusMember {
			sb.WriteString("Customer is an existing member asking about their own plan/membership. State their exact current plan name and monthly fee from the CURRENT PLAN line in LEAD context above. Then ask if they'd like to change or upgrade their plan — if yes, they can pick from the MEMBERSHIP PLANS listed above. Do not pitch a trial, they're already a member.\n")
		} else {
			sb.WriteString("Customer is asking about price. Give the exact price from the plans above. Then show the value: what they get for that price. End with a soft CTA to book a trial.\n")
		}
	case "booking_inquiry":
		sb.WriteString("Customer wants to book. Make it easy — confirm what they want and immediately offer next steps (ask for preferred date/time or provide a booking link). Be enthusiastic. Do NOT ask motivation or goal questions — they already decided to book.\n")
	case "objection":
		sb.WriteString("Customer has a concern or objection. Acknowledge it empathetically, address it with facts from the knowledge base, then redirect toward a trial (low commitment).\n")
	case "ready_to_buy":
		sb.WriteString("Customer is ready. Do NOT delay — confirm the next action clearly and immediately. Help them complete the purchase or booking right now.\n")
	case "off_topic":
		sb.WriteString("Customer is asking something unrelated to the studio. Politely acknowledge, then steer the conversation back to how the studio can help them.\n")
	default:
		if lead != nil && lead.Status == leads.StatusMember {
			sb.WriteString("Customer is an existing member. Answer their question helpfully and directly using the CURRENT PLAN and knowledge base context above. If their question is at all about their plan/membership, state their current plan and monthly fee, then ask if they'd like to change or upgrade it. Do not ask fitness-goal/qualifying questions, they've already signed up.\n")
		} else if historyAlreadyAskedAboutGoals(history) || (lead != nil && lead.Goals != "") {
			// The qualifying question is one-time, not repeated every turn.
			sb.WriteString("Answer the question helpfully and concisely. Do NOT ask about their fitness goals or motivations again — that was already asked earlier in this conversation (or is already on file). Move the conversation forward instead, e.g. toward booking a trial, without repeating that question.\n")
		} else {
			sb.WriteString("Answer the question helpfully and concisely. Then ask one qualifying question to understand their fitness goals.\n")
		}
	}

	// ── Learned studio voice (real examples + distilled style profile) ────────
	// Both are derived from this studio's own past staff-authored replies —
	// see StyleWorker and SearchStyleExamples. Empty for a studio with no
	// staff-reply history yet, in which case the generic sentiment-based
	// "Tone:" line below is all that applies, same as before this existed.
	if studio.CommunicationStyleProfile != "" {
		sb.WriteString("How " + studio.Name + "'s team communicates (learned from their own past replies):\n")
		sb.WriteString(studio.CommunicationStyleProfile + "\n\n")
	}
	if len(styleExamples) > 0 {
		sb.WriteString("Here's how " + studio.Name + "'s team has replied to similar situations before — match this voice:\n")
		for _, ex := range styleExamples {
			if ex.CustomerMessage != "" {
				sb.WriteString("- Customer: \"" + ex.CustomerMessage + "\" → Reply: \"" + ex.StudioReply + "\"\n")
			} else {
				sb.WriteString("- Reply: \"" + ex.StudioReply + "\"\n")
			}
		}
		sb.WriteString("\n")
	}

	// Sentiment overlay
	switch sentiment {
	case 1:
		if lead != nil && lead.Status == leads.StatusTrialBooked {
			sb.WriteString("Tone: enthusiastic — they're interested, push gently for membership commitment.\n")
		} else {
			sb.WriteString("Tone: enthusiastic — they're interested, push gently for a trial booking.\n")
		}
	case -1:
		sb.WriteString("Tone: empathetic — they seem hesitant. Listen first, address concerns, lower the commitment bar (trial is free/cheap).\n")
	default:
		isExistingMember := lead != nil && lead.Status == leads.StatusMember
		if intent != "booking_inquiry" && intent != "ready_to_buy" && !isExistingMember {
			sb.WriteString("Tone: curious and friendly — ask one open question to uncover their motivation.\n")
		} else if isExistingMember {
			sb.WriteString("Tone: warm and direct — they're already a member, just answer helpfully without prospecting questions.\n")
		}
	}
	sb.WriteString("Keep the reply concise: 2–4 sentences maximum. Never make up facts.\n\n")

	// Semantic history ("--- RELEVANT PAST CONTEXT ---") removed
	// (ai-reply-flow-llm-design task 3.3, per doc §9: a semantic search
	// over prior conversation turns was the source of the wrong-week bug
	// this whole change traces back to — facts now come only from the
	// studio data pack above, never from similarity-matched past
	// messages).

	// ── Prior context summary (from imported chat history, if any) ───────────
	if aiContextSummary != "" {
		sb.WriteString("--- SUMMARY OF EARLIER HISTORY (before this recent window) ---\n")
		sb.WriteString(aiContextSummary)
		sb.WriteString("\n\n")
	}

	// ── Recent conversation window ────────────────────────────────────────────
	sb.WriteString("--- RECENT CONVERSATION ---\n")
	for _, m := range history {
		if m.Direction == DirectionInbound {
			sb.WriteString(fmt.Sprintf("Customer: %s\n", m.Body))
		} else {
			sb.WriteString(fmt.Sprintf("Assistant: %s\n", m.Body))
		}
	}
	sb.WriteString("Assistant: ")

	return sb.String()
}

// isLikelyQuestion mirrors the question/short-choice heuristic
// processInboundLeadAutomation already uses (service.go) to stop a bare
// keyword match from misreading a genuine question as a definitive choice.
func isLikelyQuestion(text string) bool {
	return strings.Contains(text, "?") ||
		strings.HasPrefix(text, "what") ||
		strings.HasPrefix(text, "how") ||
		strings.HasPrefix(text, "when") ||
		strings.HasPrefix(text, "where") ||
		strings.HasPrefix(text, "why") ||
		strings.HasPrefix(text, "is ") ||
		strings.HasPrefix(text, "are ") ||
		strings.HasPrefix(text, "do ") ||
		strings.HasPrefix(text, "can ")
}

// detectOptionChoice reads a reply as a definitive trial/member choice. A bare keyword
// (outside of an explicit phrase like "book a trial") only counts when the message both
// isn't a question and is short (≤8 words) — mirrors service.go's processInboundLeadAutomation.
func (w *AIWorker) detectOptionChoice(body string, status leads.LeadStatus) (leads.LeadStatus, bool) {
	text := strings.ToLower(strings.TrimSpace(body))
	isQuestion := isLikelyQuestion(text)
	isShortChoice := !isQuestion && len(strings.Fields(text)) <= 8

	if status == leads.StatusTrialBooked {
		hasMemberKeywords := text == "1" ||
			strings.Contains(text, "become a member") ||
			(isShortChoice && (strings.Contains(text, "ready") || strings.Contains(text, "yes")))
		hasDroppedKeywords := text == "2" ||
			strings.Contains(text, "not right now") ||
			(isShortChoice && (strings.Contains(text, "no") || strings.Contains(text, "later")))

		if hasMemberKeywords && hasDroppedKeywords {
			return "", false
		}
		if hasMemberKeywords {
			return leads.StatusMember, true
		}
		if hasDroppedKeywords {
			return leads.StatusDropped, true
		}
		return "", false
	}

	hasTrialKeywords := text == "1" ||
		strings.Contains(text, "book a trial") ||
		strings.Contains(text, "book trial") ||
		strings.Contains(text, "take a trial") ||
		strings.Contains(text, "take trial") ||
		strings.Contains(text, "trial booked") ||
		strings.Contains(text, "trial booking") ||
		strings.Contains(text, "book trail") ||
		(isShortChoice && (strings.Contains(text, "trial") || strings.Contains(text, "trail"))) // "trail": common typo for "trial"

	hasMemberKeywords := text == "2" ||
		strings.Contains(text, "become a member") ||
		strings.Contains(text, "become member") ||
		strings.Contains(text, "becoming a member") ||
		strings.Contains(text, "membership") ||
		(isShortChoice && strings.Contains(text, "member"))

	// If both types of keywords are present (e.g. asking a question comparing them), it's ambiguous.
	if hasTrialKeywords && hasMemberKeywords {
		return "", false
	}

	if hasTrialKeywords {
		return leads.StatusTrialBooked, true
	}
	if hasMemberKeywords {
		return leads.StatusMember, true
	}
	return "", false
}

func (w *AIWorker) updateLeadStatus(ctx context.Context, studioID uuid.UUID, lead *leads.Lead, body string, sentiment int, confidence float64) {
	// 1. Check for explicit option choices first
	if targetStatus, ok := w.detectOptionChoice(body, lead.Status); ok {
		if lead.Status != targetStatus {
			err := w.leadsRepo.UpdateStatus(ctx, studioID, lead.ID, targetStatus)
			if err != nil {
				w.log.Error("update lead status via choice selection", "lead", lead.ID, "target", targetStatus, "err", err)
			} else {
				w.log.Info("lead status auto-updated (choice selection)", "lead", lead.ID, "from", lead.Status, "to", targetStatus)
				if targetStatus == leads.StatusTrialBooked {
					w.scheduleTrialFollowup(ctx, studioID, lead, lead.ID)
				}
			}
		}
		return
	}

	// 2. Only update if confidence is high enough
	if confidence < 0.7 {
		return
	}

	// Determine new status based on sentiment and current status
	var newStatus leads.LeadStatus
	var shouldUpdate bool

	if sentiment == 1 {
		// Positive sentiment - progress lead forward
		switch lead.Status {
		case leads.StatusNew:
			newStatus = leads.StatusContacted
			shouldUpdate = true
		case leads.StatusContacted:
			newStatus = leads.StatusTrialBooked
			shouldUpdate = true
		case leads.StatusTrialBooked:
			newStatus = leads.StatusMember
			shouldUpdate = true
		case leads.StatusMember, leads.StatusDropped:
			// Already at final status
			shouldUpdate = false
		}
	} else if sentiment == -1 {
		// Negative sentiment - mark as dropped (unless already completed)
		switch lead.Status {
		case leads.StatusMember:
			// Don't drop members
			shouldUpdate = false
		default:
			newStatus = leads.StatusDropped
			shouldUpdate = true
		}
	} else {
		// Neutral - only move if New
		if lead.Status == leads.StatusNew {
			newStatus = leads.StatusContacted
			shouldUpdate = true
		}
	}

	if !shouldUpdate {
		return
	}

	err := w.leadsRepo.UpdateStatus(ctx, studioID, lead.ID, newStatus)
	if err != nil {
		w.log.Error("update lead status", "lead", lead.ID, "current", lead.Status, "new", newStatus, "err", err)
	} else {
		w.log.Info("lead status auto-updated", "lead", lead.ID, "from", lead.Status, "to", newStatus, "sentiment", sentiment, "confidence", confidence)
		if newStatus == leads.StatusTrialBooked {
			w.scheduleTrialFollowup(ctx, studioID, lead, lead.ID)
		}
	}
}

func (w *AIWorker) scheduleTrialFollowup(ctx context.Context, studioID uuid.UUID, lead *leads.Lead, fallbackConvID uuid.UUID) {
	// Try to find the actual conversation ID for this lead to avoid any mismatch.
	convID := fallbackConvID
	err := w.msgRepo.Pool().QueryRow(ctx, `
		SELECT id FROM conversations 
		WHERE studio_id = $1 AND lead_id = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`, studioID, lead.ID).Scan(&convID)
	if err != nil {
		w.log.Warn("could not find conversation for lead to schedule followup", "lead", lead.ID, "err", err)
	}

	body := "Hi {{contact.first_name}}, we hope you're enjoying your trial! Are you ready to take the next step and become a member? Please select an option:\n1. Yes, I am ready to become a member!\n2. Not right now"

	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: convID,
		Body:           body,
		SourceKind:     SourceAutomation,
		SourceRef:      fmt.Sprintf("lead:%s:trial_followup:1day", lead.ID.String()),
		ScheduledFor:   time.Now().UTC().Add(24 * time.Hour),
	}); err != nil {
		w.log.Error("enqueue 1-day trial followup failed", "lead", lead.ID, "err", err)
	}
}

// leadingTimeGreeting matches an opening "Good morning/afternoon/evening/night" plus the
// punctuation after it.
var leadingTimeGreeting = regexp.MustCompile(`(?i)^\s*good\s+(morning|afternoon|evening|night)\b[\s,!.:;-]*`)

// stripTimeGreeting removes a leading time-of-day greeting the model added despite being
// told not to. Never returns an empty reply: one that's only a greeting is left as is.
func stripTimeGreeting(resp string) string {
	loc := leadingTimeGreeting.FindStringIndex(resp)
	if loc == nil {
		return resp
	}
	rest := strings.TrimLeft(resp[loc[1]:], " \t")
	if rest == "" {
		return resp
	}
	r, size := utf8.DecodeRuneInString(rest)
	return string(unicode.ToUpper(r)) + rest[size:]
}

// leadingNameGreeting matches a leading "Hi X!" / "Hey X," / "Hello X", X being a
// capitalized word (optionally two, e.g. "Hi John Smith!"). Not tied to the lead's stored
// name — the model often greets with an invented friendlier variant instead of echoing it
// verbatim. (?i) is scoped to just the greeting word so [A-Z] still means "capitalized",
// distinguishing a real name from an ordinary word like "trainer". The name is letters-only
// so a trailing "'s" (e.g. "Hi Puneeth's trainer...") blocks the match instead of being
// misread as a greeting.
var leadingNameGreeting = regexp.MustCompile(`^\s*(?i:hi|hey|hello)\s+[A-Z][a-zA-Z]*(?:\s+[A-Z][a-zA-Z]*)?([\s,!.:;-]|$)`)

// stripNameGreeting removes a leading name-greeting the model added despite being told not
// to re-greet mid-conversation. Never returns an empty reply.
func stripNameGreeting(resp string) string {
	loc := leadingNameGreeting.FindStringIndex(resp)
	if loc == nil {
		return resp
	}
	rest := strings.TrimLeft(resp[loc[1]:], " \t")
	if rest == "" {
		return resp
	}
	r, size := utf8.DecodeRuneInString(rest)
	return string(unicode.ToUpper(r)) + rest[size:]
}

// unfilledPlaceholder matches a short square-bracket span like
// "[Insert Address Here]", "[Your Name]", "[Address]", "[TBD]" — a template
// artifact the model left in instead of either answering from the knowledge
// base or deferring to a team member (the prompt tells it to do the latter
// for anything not in the KB, but models don't follow that reliably — same
// reasoning as every other strip* guard in this file). Real WhatsApp replies
// essentially never contain literal square brackets, so any short bracketed
// span is almost certainly this, not legitimate content.
var unfilledPlaceholder = regexp.MustCompile(`\[[^\[\]]{1,60}\]`)

// guardAgainstPlaceholderArtifact catches an AI reply that leaked an unfilled
// template placeholder (e.g. "...you can find us at [Insert Address Here]")
// straight to a real customer — confirmed in production. A surgical removal
// would leave a half-sentence ("you can find us at ."), which reads just as
// broken, so the whole reply is swapped for a safe, generic fallback instead
// — the model clearly didn't have the real answer, so handing off to a human
// is the correct behavior anyway, matching what the prompt already asks for
// when information is missing.
func guardAgainstPlaceholderArtifact(resp string) string {
	if !unfilledPlaceholder.MatchString(resp) {
		return resp
	}
	return "Let me get that confirmed for you — one of our team members will follow up shortly with the details!"
}

// motivationPhrases are the fitness-goal/qualifying-question phrasings the prompt
// asks the model to use. Shared by stripMotivationQuestions (strips them from a
// reply) and historyAlreadyAskedAboutGoals (detects that one was already asked
// earlier in this conversation, so buildPrompt stops instructing the model to ask
// it again on every single turn — the qualifying question is a one-time thing, not
// a tic to repeat on every reply).
var motivationPhrases = []string{
	"what motivated you",
	"what are your fitness goals",
	"what are your main fitness goals",
	"what are your goals",
	"goals or motivations",
	"are you looking to lose weight",
	"are you looking to gain",
	"gain strength",
	"improve overall health",
	"why do you want to",
	"tell me what motivated",
	"before we get started, can you tell me",
	"before we proceed, can you tell",
	"before we book",
	"before we do that, can you tell",
}

// historyAlreadyAskedAboutGoals reports whether an earlier outbound message in this
// conversation already asked the customer about their fitness goals/motivations —
// so the prompt should not instruct the model to ask it again (and the post-
// processing strip below applies even outside the booking_inquiry intent).
func historyAlreadyAskedAboutGoals(history []Message) bool {
	for _, m := range history {
		if m.Direction != DirectionOutbound {
			continue
		}
		lower := strings.ToLower(m.Body)
		for _, phrase := range motivationPhrases {
			if strings.Contains(lower, phrase) {
				return true
			}
		}
	}
	return false
}

// stripMotivationQuestions removes sentences asking about fitness goals/motivations
// when the customer has already expressed intent to book, or already been asked
// earlier in this same conversation — the LLM ignores the prompt instruction
// reliably, so we enforce it at the output layer.
func stripMotivationQuestions(resp string) string {
	lower := strings.ToLower(resp)
	hasMotivation := false
	for _, phrase := range motivationPhrases {
		if strings.Contains(lower, phrase) {
			hasMotivation = true
			break
		}
	}
	if !hasMotivation {
		return resp
	}

	// Split on ". " and filter out offending sentences.
	parts := strings.Split(resp, ". ")
	var kept []string
	for _, part := range parts {
		partLower := strings.ToLower(part)
		remove := false
		for _, phrase := range motivationPhrases {
			if strings.Contains(partLower, phrase) {
				remove = true
				break
			}
		}
		if !remove {
			kept = append(kept, part)
		}
	}

	result := strings.TrimSpace(strings.Join(kept, ". "))
	if result == "" {
		return "I'd be happy to book a trial for you! What date and time works best for you?"
	}
	return result
}
