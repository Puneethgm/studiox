package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/projectx/api/internal/decisiontree"
	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/embeddings"
	"github.com/projectx/api/internal/integrations/gemini"
	"github.com/projectx/api/internal/integrations/groq"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/studios"
)

const (
	aiResyncInterval = 30 * time.Second
)

type AIWorker struct {
	bus          Bus
	msgRepo      *Repo
	msgSvc       *Service
	studiosRepo  *studios.Repo
	leadsRepo    *leads.Repo
	dtSvc        *decisiontree.Service
	claude       *claude.Client
	claudeAPIURL string
	geminiClient *gemini.Client
	embeddings   *embeddings.Client
	llmRepo      *llm.Repo
	log          *slog.Logger
	subs         map[uuid.UUID]func()
	httpClient   *http.Client
}

func NewAIWorker(bus Bus, msgRepo *Repo, msgSvc *Service, studiosRepo *studios.Repo, leadsRepo *leads.Repo, dtSvc *decisiontree.Service, cl *claude.Client, claudeAPIURL string, embClient *embeddings.Client, llmRepo *llm.Repo, log *slog.Logger) *AIWorker {
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
		bus:          bus,
		msgRepo:      msgRepo,
		msgSvc:       msgSvc,
		studiosRepo:  studiosRepo,
		leadsRepo:    leadsRepo,
		dtSvc:        dtSvc,
		claude:       cl,
		claudeAPIURL: claudeAPIURL,
		geminiClient: gemini.New(),
		embeddings:   embClient,
		llmRepo:      llmRepo,
		log:          log,
		subs:         make(map[uuid.UUID]func()),
		httpClient:   client,
	}
}

func (w *AIWorker) Run(ctx context.Context) {
	if w.claude == nil {
		w.log.Info("claude not configured; ai worker will run using studio-configured Gemini API keys where available")
	} else {
		w.log.Info("claude configured; starting ai worker")
	}
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
		go w.listenStudio(ctx, s.ID, ch)
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
				go func(msgID uuid.UUID) {
					if err := w.handleMessage(ctx, studioID, msgID); err != nil {
						w.log.Error("ai handle message", "err", err)
					}
				}(*evt.MessageID)
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
	history, err := w.msgRepo.ListMessages(ctx, studioID, convID, 10)
	if err != nil {
		return fmt.Errorf("fetch message history: %w", err)
	}
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
	return llmWaterfall(ctx, w.studiosRepo, w.llmRepo, w.msgRepo, w.claude, w.claudeAPIURL, w.log, studioID, studio, prompt)
}

// llmWaterfall tries Groq → Gemini → Claude in order, falling through on
// error or an empty reply, and logs every attempt via msgRepo.LogLLMUsage.
// This is the platform's one shared "give me a completion for this prompt,
// I don't care which provider" path — used both for AI reply generation
// (via runSummaryWaterfall above) and for any other background task that
// needs an LLM call without depending on a specific provider being
// configured (see StyleWorker, which can't assume Claude is set up since
// it's a single platform-wide key while Groq/Gemini are per-studio).
//
// Which MODEL each provider tries is read from studio_ai_models via
// llmRepo.EnabledModelsForStudio, not hardcoded — a studio's AI Assistant
// settings page controls this list (internal/studios/ai_models_http.go). A
// provider with several enabled models tries each in order (first success
// wins), same as Groq's small-then-large fallback used to be hardcoded.
func llmWaterfall(ctx context.Context, studiosRepo *studios.Repo, llmRepo *llm.Repo, msgRepo *Repo, claudeClient *claude.Client, claudeAPIURL string, log *slog.Logger, studioID uuid.UUID, studio *studios.Studio, prompt string) (text string, sourceRef string) {
	groqKey := studio.GroqAPIKey
	if groqKey == "" {
		if pk, e := studiosRepo.GetPlatformSetting(ctx, "groq_api_key"); e == nil {
			groqKey = pk
		}
	}
	if groqKey != "" {
		groqClient := groq.New(groqKey)
		models, _ := llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderGroq)
		for _, model := range models {
			t0 := time.Now()
			gr, gerr := groqClient.GenerateReply(ctx, prompt, model)
			latMs := int(time.Since(t0).Milliseconds())
			errMsg := ""
			if gerr != nil {
				errMsg = gerr.Error()
			}
			ok := gerr == nil && strings.TrimSpace(gr.Text) != ""
			msgRepo.LogLLMUsage(ctx, studioID, "groq", model, latMs, ok, errMsg, gr.TokensIn, gr.TokensOut)
			if ok {
				return gr.Text, "groq:" + model
			}
		}
	}

	geminiKey := studio.GeminiAPIKey
	if geminiKey == "" {
		if pk, e := studiosRepo.GetPlatformSetting(ctx, "gemini_api_key"); e == nil {
			geminiKey = pk
		}
	}
	if geminiKey != "" {
		models, _ := llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderGemini)
		for _, model := range models {
			t0 := time.Now()
			gemReply, gerr := gemini.New().GenerateReplyForModel(ctx, geminiKey, model, prompt)
			latMs := int(time.Since(t0).Milliseconds())
			errMsg := ""
			if gerr != nil {
				errMsg = gerr.Error()
			}
			ok := gerr == nil && gemReply.Text != ""
			msgRepo.LogLLMUsage(ctx, studioID, "gemini", model, latMs, ok, errMsg, gemReply.TokensIn, gemReply.TokensOut)
			if ok {
				return gemReply.Text, "gemini:" + model
			}
		}
	}

	claudeCl := claudeClient
	if studio.ClaudeAPIKey != "" {
		if c, cerr := claude.New(claudeAPIURL, studio.ClaudeAPIKey); cerr == nil && c != nil {
			claudeCl = c
		}
	}
	if claudeCl != nil {
		models, _ := llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderClaude)
		for _, model := range models {
			t0 := time.Now()
			cr, cerr := claudeCl.GenerateReplyForModel(ctx, prompt, model)
			latMs := int(time.Since(t0).Milliseconds())
			errMsg := ""
			if cerr != nil {
				errMsg = cerr.Error()
			}
			ok := cerr == nil && cr.Text != ""
			msgRepo.LogLLMUsage(ctx, studioID, "claude", model, latMs, ok, errMsg, cr.TokensIn, cr.TokensOut)
			if ok {
				return cr.Text, "claude:" + model
			}
		}
	}

	return "", ""
}

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

	// Keyword sentiment as initial fallback
	sentiment, confidence, keywords := w.analyzeSentiment(msg.Body)

	// Resolve Gemini API key (studio key takes priority over platform key)
	apiKey := studio.GeminiAPIKey
	if apiKey == "" {
		if platformKey, err := w.studiosRepo.GetPlatformSetting(ctx, "gemini_api_key"); err == nil && platformKey != "" {
			apiKey = platformKey
		}
	}
	// Single model pick (no waterfall needed here) for the classification/
	// expansion/rerank helper calls below — studio-configured via the AI
	// Assistant settings page, falling back to gemini.Models[0].
	geminiModel := ""
	if models, mErr := w.llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderGemini); mErr == nil && len(models) > 0 {
		geminiModel = models[0]
	}

	// Fetch last 15 messages as the immediate recent window. 5 was too
	// short — the model would forget questions/offers it made only a few
	// turns back and repeat itself.
	history, err := w.msgRepo.ListMessages(ctx, studioID, conv.ID, 15)
	if err != nil {
		w.log.Error("fetch message history for ai context failed", "err", err)
		history = []Message{*msg}
	}

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
	var semanticHistory []SemanticMatch
	var styleExamples []StyleExample
	var intent string
	// kbConfident tracks whether retrieval found high-confidence chunks.
	// Used to gate hallucination: if false, the prompt instructs the AI not to guess.
	kbConfident := false

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
		if err := w.msgRepo.EscalateConversation(ctx, studioID, conv.ID, "Customer asked to speak with a human"); err != nil {
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

	// Keyword override: runs before LLM classification so it works even when Gemini is not configured.
	// "trail" is a typo for "trial" that LLMs classify as hiking/off-topic.
	if strings.Contains(lowerBody, "trail") ||
		strings.Contains(lowerBody, "book trial") || strings.Contains(lowerBody, "book a trial") {
		intent = "booking_inquiry"
		w.log.Debug("intent set to booking_inquiry by keyword", "message_id", msg.ID)
	}

	if apiKey != "" && msg.Body != "" {
		// Step 1+2 in parallel: classify intent and expand query simultaneously.
		// Neither depends on the other so we save one full LLM round-trip.
		type classifyResult struct {
			intent    string
			sentiment int
			conf      float64
		}
		classifyCh := make(chan classifyResult, 1)
		expandCh := make(chan string, 1)
		go func() {
			i, s, c := studios.ClassifyIntent(ctx, apiKey, geminiModel, msg.Body)
			classifyCh <- classifyResult{i, s, c}
		}()
		go func() {
			expandCh <- studios.ExpandQuery(ctx, apiKey, geminiModel, msg.Body)
		}()

		cr := <-classifyCh
		if intent == "" {
			intent = cr.intent
		}
		sentiment = cr.sentiment
		confidence = cr.conf
		keywords = []string{intent}
		// Re-apply keyword override in case LLM classification overwrote it.
		if strings.Contains(lowerBody, "trail") ||
			strings.Contains(lowerBody, "book trial") || strings.Contains(lowerBody, "book a trial") {
			intent = "booking_inquiry"
		}

		expandedQuery := <-expandCh

		// Step 3: Embed the expanded query.
		queryVec, err := w.embeddings.EmbedQuery(ctx, expandedQuery)
		if err == nil {
			// Step 3b: Persist original message embedding fully async — not needed for this response.
			capturedIntent, capturedSentiment, capturedConf := intent, sentiment, confidence
			go func() {
				saveCtx := context.Background()
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

			// Step 4+8 in parallel: KB retrieval/rerank and semantic history simultaneously.
			type kbResult struct {
				chunks    []string
				confident bool
			}
			kbCh := make(chan kbResult, 1)
			semHistCh := make(chan []SemanticMatch, 1)
			styleCh := make(chan []StyleExample, 1)

			go func() {
				// Hybrid retrieval (8 candidates) → rerank to top 4 directly.
				// MMR diversity pass removed: saves 4-6 sequential embedding API calls per request.
				matched, searchErr := w.studiosRepo.SearchKnowledgeChunksHybrid(ctx, studioID, queryVec, expandedQuery, "all", 8)
				if searchErr != nil || len(matched) == 0 {
					if searchErr != nil {
						w.log.Warn("failed to search knowledge chunks", "studio_id", studioID, "err", searchErr)
					}
					kbCh <- kbResult{}
					return
				}
				contents := make([]string, len(matched))
				for i, m := range matched {
					contents[i] = m.Content
				}
				reranked := studios.RerankChunks(ctx, apiKey, geminiModel, msg.Body, contents, 4)
				kbCh <- kbResult{chunks: reranked, confident: len(reranked) >= 2}
			}()

			go func() {
				recentIDs := make([]uuid.UUID, len(history))
				for i, m := range history {
					recentIDs[i] = m.ID
				}
				semHist, semErr := w.msgRepo.SearchSemanticHistory(ctx, studioID, conv.ID, queryVec, recentIDs, 3)
				if semErr != nil || len(semHist) == 0 {
					semHistCh <- nil
					return
				}
				var filtered []SemanticMatch
				for _, sm := range semHist {
					if sm.Score >= 0.75 {
						filtered = append(filtered, sm)
					}
				}
				semHistCh <- filtered
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

			if semResults := <-semHistCh; len(semResults) > 0 {
				semanticHistory = semResults
				w.log.Info("retrieved semantic history", "studio_id", studioID, "count", len(semanticHistory))
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
				if lead != nil {
					reply = strings.ReplaceAll(reply, "{{lead_name}}", lead.Name)
					reply = strings.ReplaceAll(reply, "{{lead_first_name}}", lead.FirstName)
					reply = strings.ReplaceAll(reply, "{{lead_status}}", string(lead.Status))
				}
				if studio != nil {
					reply = strings.ReplaceAll(reply, "{{studio_name}}", studio.Name)
				}
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
				if err := w.msgRepo.EscalateConversation(ctx, studioID, msg.ConversationID, treeResult.NodeLabel); err != nil {
					w.log.Warn("failed to mark conversation escalated", "studio_id", studioID, "err", err)
				}
				if treeResult.Reply != "" {
					reply := treeResult.Reply
					if lead != nil {
						reply = strings.ReplaceAll(reply, "{{lead_name}}", lead.Name)
						reply = strings.ReplaceAll(reply, "{{lead_first_name}}", lead.FirstName)
					}
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
				if lead != nil {
					bookBody = strings.ReplaceAll(bookBody, "{{lead_name}}", lead.Name)
					bookBody = strings.ReplaceAll(bookBody, "{{lead_first_name}}", lead.FirstName)
				}
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
				if lead != nil {
					reply = strings.ReplaceAll(reply, "{{lead_name}}", lead.Name)
					reply = strings.ReplaceAll(reply, "{{lead_first_name}}", lead.FirstName)
				}
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

	// "Yes to trial" shortcut: skip AI and send the payment link directly
	// when either (a) the customer explicitly asks to buy/pay — that alone
	// is unambiguous, no prior offer needed — or (b) they give a bare
	// affirmation ("yes"/"keen") replying to a bot message that had just
	// offered a trial.
	if lead != nil {
		lowerMsg := strings.ToLower(strings.TrimSpace(msg.Body))
		explicitPurchaseIntent := strings.Contains(lowerMsg, "buy") ||
			strings.Contains(lowerMsg, "purchase") ||
			strings.Contains(lowerMsg, "how to pay") ||
			strings.Contains(lowerMsg, "how do i pay") ||
			strings.Contains(lowerMsg, "payment link") ||
			strings.Contains(lowerMsg, "pay for") ||
			strings.Contains(lowerMsg, "checkout")
		isAffirmative := lowerMsg == "yes" || lowerMsg == "yeah" || lowerMsg == "sure" ||
			lowerMsg == "ok" || lowerMsg == "okay" || lowerMsg == "yep" || lowerMsg == "yup" ||
			lowerMsg == "y" || lowerMsg == "keen" || lowerMsg == "im keen" || lowerMsg == "i'm keen" ||
			strings.HasPrefix(lowerMsg, "yes ") || strings.HasPrefix(lowerMsg, "sure ") ||
			strings.Contains(lowerMsg, "i'm keen") || strings.Contains(lowerMsg, "im keen") ||
			strings.Contains(lowerMsg, "interested")

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

	// Don't guess on a factual question we don't have solid grounding for —
	// hand off to a human instead of generating an unfounded answer. Scoped
	// to pricing/general factual questions specifically: small talk,
	// objections, and booking-flow messages continue through the AI as
	// normal even with a low knowledge-base match, since those don't
	// depend on KB grounding to answer well and would escalate needlessly
	// otherwise (e.g. "thanks!" always has kbConfident=false).
	if !kbConfident && (intent == "pricing_question" || intent == "general_question") {
		w.log.Info("ai worker: low KB confidence on factual question, escalating instead of guessing",
			"studio_id", studioID, "conversation_id", conv.ID, "intent", intent)
		if err := w.msgRepo.EscalateConversation(ctx, studioID, conv.ID, "AI uncertain — insufficient knowledge base match"); err != nil {
			w.log.Warn("failed to mark conversation escalated (low confidence)", "studio_id", studioID, "err", err)
		} else {
			handoffBody := "That's a great question — let me get one of our team members to help you with the details. They'll be with you shortly!"
			if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
				StudioID:       studioID,
				ConversationID: conv.ID,
				Body:           handoffBody,
				SourceKind:     SourceAI,
				SourceRef:      "low_confidence_handoff",
				ScheduledFor:   time.Now().UTC().Add(replyDelay),
			}); err != nil {
				w.log.Error("failed to enqueue low-confidence handoff message", "err", err, "conv_id", conv.ID)
			} else {
				w.bus.Publish(ctx, Event{
					Kind:           EvtOutboundJobEnqueued,
					StudioID:       studioID,
					ConversationID: conv.ID,
				})
			}
		}
		return nil
	}

	// Generate AI response with context
	aiContextSummary, err := w.msgRepo.GetConversationAISummary(ctx, studioID, conv.ID)
	if err != nil {
		w.log.Warn("fetch conversation ai summary failed", "err", err)
		aiContextSummary = ""
	}
	prompt := w.buildPrompt(ctx, history, semanticHistory, styleExamples, conv, lead, studio, plans, sentiment, keywords, kbChunks, intent, kbConfident, aiContextSummary)

	// Waterfall: Groq → Gemini → Claude. Which model(s) each provider tries
	// is read from studio_ai_models (studio's AI Assistant settings page),
	// not hardcoded — see llmWaterfall's doc comment.
	var resp string
	var sourceRef string

	// 1. Try Groq (fast + cheap) — use studio key first, fall back to platform key
	groqKey := studio.GroqAPIKey
	if groqKey == "" {
		if pk, e := w.studiosRepo.GetPlatformSetting(ctx, "groq_api_key"); e == nil {
			groqKey = pk
		}
	}
	if groqKey != "" {
		groqClient := groq.New(groqKey)
		groqModels, _ := w.llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderGroq)
		for _, model := range groqModels {
			w.log.Info("generating ai reply using groq", "studio_id", studioID, "message_id", msg.ID, "model", model)
			t0 := time.Now()
			gr, gerr := groqClient.GenerateReply(ctx, prompt, model)
			latMs := int(time.Since(t0).Milliseconds())
			errMsg := ""
			if gerr != nil {
				errMsg = gerr.Error()
			}
			// A short reply is treated as a soft failure (Groq's small models
			// sometimes truncate) so the next enabled model gets a turn.
			ok := gerr == nil && len(strings.TrimSpace(gr.Text)) >= 15
			w.msgRepo.LogLLMUsage(ctx, studioID, "groq", model, latMs, ok, errMsg, gr.TokensIn, gr.TokensOut)
			if ok {
				resp = gr.Text
				sourceRef = "groq:" + model
				break
			}
			w.log.Info("groq model failed or short, trying next", "studio_id", studioID, "model", model, "err", gerr)
		}
	}

	// 2. Gemini fallback
	if resp == "" && apiKey != "" {
		geminiModels, _ := w.llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderGemini)
		for _, model := range geminiModels {
			w.log.Info("generating ai reply using gemini", "studio_id", studioID, "message_id", msg.ID, "model", model)
			t0 := time.Now()
			gemReply, gerr := w.geminiClient.GenerateReplyForModel(ctx, apiKey, model, prompt)
			latMs := int(time.Since(t0).Milliseconds())
			errMsg := ""
			if gerr != nil {
				errMsg = gerr.Error()
			}
			ok := gerr == nil && gemReply.Text != ""
			w.msgRepo.LogLLMUsage(ctx, studioID, "gemini", model, latMs, ok, errMsg, gemReply.TokensIn, gemReply.TokensOut)
			if ok {
				resp = gemReply.Text
				sourceRef = "gemini:" + model
				break
			}
		}
	}

	// 3. Claude fallback — studio's own key if set, else the platform client
	claudeClient := w.claude
	if studio.ClaudeAPIKey != "" {
		if c, cerr := claude.New(w.claudeAPIURL, studio.ClaudeAPIKey); cerr == nil && c != nil {
			claudeClient = c
		}
	}
	if resp == "" && claudeClient != nil {
		claudeModels, _ := w.llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderClaude)
		for _, model := range claudeModels {
			w.log.Info("generating ai reply using claude", "studio_id", studioID, "message_id", msg.ID, "model", model)
			t0 := time.Now()
			cr, cerr := claudeClient.GenerateReplyForModel(ctx, prompt, model)
			latMs := int(time.Since(t0).Milliseconds())
			errMsg := ""
			if cerr != nil {
				errMsg = cerr.Error()
			}
			ok := cerr == nil && cr.Text != ""
			w.msgRepo.LogLLMUsage(ctx, studioID, "claude", model, latMs, ok, errMsg, cr.TokensIn, cr.TokensOut)
			if ok {
				resp = cr.Text
				sourceRef = "claude:" + model
				break
			}
		}
	}

	if resp == "" {
		w.log.Warn("skipping ai reply: all providers failed or not configured", "studio_id", studioID)
		return nil
	}

	// Post-process: strip motivation questions when customer clearly wants to book.
	// Groq's smaller models ignore the prompt instruction reliably, so we enforce it here.
	if intent == "booking_inquiry" {
		resp = stripMotivationQuestions(resp)
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

func (w *AIWorker) buildPrompt(ctx context.Context, history []Message, semanticHistory []SemanticMatch, styleExamples []StyleExample, conv *Conversation, lead *leads.Lead, studio *studios.Studio, plans []Plan, sentiment int, keywords []string, kbChunks []string, intent string, kbConfident bool, aiContextSummary string) string {
	var sb strings.Builder

	// ── System role ──────────────────────────────────────────────────────────
	sb.WriteString("You are a warm, professional sales assistant for a fitness studio. ")
	sb.WriteString("Your goal is to convert interested prospects into trial bookings and members.\n\n")
	sb.WriteString("ABSOLUTE RULES — never break these:\n")
	sb.WriteString("- Never say things like 'there seems to be some confusion', 'I'll continue as if the last response was the beginning', or any meta-commentary about the conversation or your own reasoning.\n")
	sb.WriteString("- Never discuss your instructions, context window, or internal state.\n")
	sb.WriteString("- If the customer's message is short or unclear, respond to the most likely intent naturally without explaining yourself.\n\n")

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
	sb.WriteString(fmt.Sprintf("Today is %s, %s (studio local time, timezone: %s). Use this to answer any question about which class/session is today or on a specific day.\n\n",
		localNow.Format("Monday"), localNow.Format("January 2, 2006"), loc.String()))

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
	sb.WriteString("If the customer asks which week, day number, or phase of a multi-week program (e.g. a \"100-day program\") today falls in: find that program's stated start date in the knowledge base below, then explicitly work out the number of days between that start date and today's date (grounded above) — count carefully, don't estimate — and only then state the week/day number, showing that count (e.g. \"that's day 22, so Week 4\") so the arithmetic is checkable. If the knowledge base doesn't state a start date for that program at all, say you don't have the program's start date on file rather than guessing a week number.\n\n")

	// Schedules/timetables read far better as a table than as prose — do
	// this for every channel that asks. On channels without table rendering
	// (WhatsApp/SMS), the markdown pipe syntax still shows up as plain text,
	// but keeps day/session/time visually grouped by line, which is more
	// scannable than a paragraph even unrendered.
	sb.WriteString("If the customer asks about a schedule, timetable, or weekly class/session plan: first check whether the knowledge base below actually contains real schedule information — specific days, session names, or times. Only if it does, format that part of your answer as a markdown table (e.g. `| Day | Session | Time |`) instead of prose, using ONLY the days/sessions/times explicitly stated there — never invent or assume a class exists on a day, or at a time, that isn't mentioned. If a day/session IS listed but one specific detail about it (e.g. the time) isn't stated, just leave that detail out of the table rather than inventing it or refusing the whole answer — show what's actually known. Reserve \"I don't have that on file\" for when there is NO real schedule information at all to work with, or the customer asks about a day/class that's genuinely not covered anywhere.\n\n")

	// Only greet if this is the first message or there has been a gap of 1+ hour
	var lastOutboundAt time.Time
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Direction == DirectionOutbound {
			lastOutboundAt = history[i].SentAt
			break
		}
	}
	isFirstContact := lastOutboundAt.IsZero()
	isLongGap := !lastOutboundAt.IsZero() && now.Sub(lastOutboundAt) > time.Hour

	if isFirstContact || isLongGap {
		hour := localNow.Hour()
		greeting := "Good evening"
		if hour < 12 {
			greeting = "Good morning"
		} else if hour < 17 {
			greeting = "Good afternoon"
		}
		sb.WriteString(fmt.Sprintf("Open your reply with '%s'. ", greeting))
	} else {
		sb.WriteString("Do NOT start with a greeting like Good morning/afternoon/evening — jump straight into the response. ")
	}
	sb.WriteString("\n\n")

	// ── Knowledge base ───────────────────────────────────────────────────────
	kbText := ""
	if len(kbChunks) > 0 {
		kbText = strings.Join(kbChunks, "\n\n")
	} else if studio != nil {
		kbText = studio.KnowledgeBase
		for _, f := range studio.KnowledgeBaseFiles {
			if f.Text != "" {
				kbText += fmt.Sprintf("\n\nDocument (%s):\n%s", f.Name, f.Text)
			}
		}
	}

	if kbText != "" {
		sb.WriteString("KNOWLEDGE BASE:\n\"\"\"\n")
		sb.WriteString(kbText)
		sb.WriteString("\n\"\"\"\n")
		sb.WriteString("Use the knowledge base above to answer factual questions. If it doesn't fully cover the exact question, answer helpfully using what IS there — but never invent a specific fact (a day, time, price, date, or policy) that isn't actually stated above. If the knowledge base above is unrelated to what the customer is asking (e.g. it's about something else entirely, not this topic), treat that the same as having no information — do not use it as a basis to construct a plausible-sounding invented answer. If a specific detail truly isn't in the knowledge base, say so plainly (e.g. \"I don't have that on file\") rather than guessing or making one up — that's more useful to the customer than a confident wrong answer, and better than a blanket \"someone will follow up\" for something you could otherwise answer. ")
		sb.WriteString("You can see your own earlier replies in RECENT CONVERSATION below — do NOT restate pricing, promotions, or programme details you've already told the customer in this conversation; assume they remember it and only repeat something if they explicitly ask again. Keep replies short and move the conversation forward instead of re-explaining what's already covered.\n\n")
	}

	// ── Plans ────────────────────────────────────────────────────────────────
	if len(plans) > 0 {
		sb.WriteString("MEMBERSHIP PLANS (SGD):\n")
		for _, p := range plans {
			sb.WriteString(fmt.Sprintf("  • %s — S$%.2f/%s | %s\n",
				p.PlanName, float64(p.PriceSGD)/100.0, p.BillingCycle, strings.Join(p.Features, ", ")))
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
				if len(history) > 0 {
					lastMsg := history[len(history)-1]
					if lastMsg.Direction == DirectionInbound {
						lt := strings.ToLower(lastMsg.Body)
						customerAsksAboutBooking = strings.Contains(lt, "book") ||
							strings.Contains(lt, "trial") ||
							strings.Contains(lt, "trail") || // common typo for "trial"
							strings.Contains(lt, "join") ||
							strings.Contains(lt, "member") ||
							strings.Contains(lt, "sign up") ||
							strings.Contains(lt, "enroll") ||
							strings.Contains(lt, "register")
					}
				}
				if !optionsAlreadySent || customerAsksAboutBooking {
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

	// ── Semantic history (high-confidence past context) ───────────────────────
	if len(semanticHistory) > 0 {
		sb.WriteString("--- RELEVANT PAST CONTEXT (earlier messages semantically similar to today's query) ---\n")
		for _, sm := range semanticHistory {
			label := "Assistant (earlier)"
			if sm.Direction == DirectionInbound {
				label = "Customer (earlier)"
			}
			sb.WriteString(fmt.Sprintf("%s [similarity %.2f]: %s\n", label, sm.Score, sm.Body))
		}
		sb.WriteString("\n")
	}

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

func (w *AIWorker) detectOptionChoice(body string, status leads.LeadStatus) (leads.LeadStatus, bool) {
	text := strings.ToLower(strings.TrimSpace(body))

	if status == leads.StatusTrialBooked {
		hasMemberKeywords := text == "1" ||
			strings.Contains(text, "become a member") ||
			strings.Contains(text, "ready") ||
			strings.Contains(text, "yes")
		hasDroppedKeywords := text == "2" ||
			strings.Contains(text, "not right now") ||
			strings.Contains(text, "no") ||
			strings.Contains(text, "later")

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
		strings.Contains(text, "trial") ||
		strings.Contains(text, "book trail") ||
		strings.Contains(text, "trail") // common typo for "trial"

	hasMemberKeywords := text == "2" ||
		strings.Contains(text, "become a member") ||
		strings.Contains(text, "become member") ||
		strings.Contains(text, "becoming a member") ||
		strings.Contains(text, "membership") ||
		strings.Contains(text, "member")

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

// stripMotivationQuestions removes sentences asking about fitness goals/motivations
// when the customer has already expressed intent to book — the LLM ignores the prompt
// instruction reliably, so we enforce it at the output layer.
func stripMotivationQuestions(resp string) string {
	motivationPhrases := []string{
		"what motivated you",
		"what are your fitness goals",
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
