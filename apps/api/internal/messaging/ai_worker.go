package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/projectx/api/internal/decisiontree"
	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/groq"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/studios"
)

type claudeReply = claude.Reply

type geminiReply struct {
	text      string
	tokensIn  int
	tokensOut int
}

const (
	aiResyncInterval = 30 * time.Second
)

type AIWorker struct {
	bus         Bus
	msgRepo     *Repo
	msgSvc      *Service
	studiosRepo *studios.Repo
	leadsRepo   *leads.Repo
	dtSvc       *decisiontree.Service
	claude      *claude.Client
	log         *slog.Logger
	subs        map[uuid.UUID]func()
	httpClient  *http.Client
}

func NewAIWorker(bus Bus, msgRepo *Repo, msgSvc *Service, studiosRepo *studios.Repo, leadsRepo *leads.Repo, dtSvc *decisiontree.Service, cl *claude.Client, log *slog.Logger) *AIWorker {
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
		bus:         bus,
		msgRepo:     msgRepo,
		msgSvc:      msgSvc,
		studiosRepo: studiosRepo,
		leadsRepo:   leadsRepo,
		dtSvc:       dtSvc,
		claude:      cl,
		log:         log,
		subs:        make(map[uuid.UUID]func()),
		httpClient:  client,
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
			if evt.Kind != EvtMessageReceived || evt.MessageID == nil {
				continue
			}
			go func(msgID uuid.UUID) {
				if err := w.handleMessage(ctx, studioID, msgID); err != nil {
					w.log.Error("ai handle message", "err", err)
				}
			}(*evt.MessageID)
		}
	}
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
		} else {
			// Skip AI only for mid-flow stages where the bot owns the conversation
			// (date/time selection, plan selection, reason collection).
			// At awaiting_interest and awaiting_options the bot only reacts to exact
			// numeric/keyword choices; anything else (questions, greetings) falls to AI.
			stage := lead.AutoContactStage
			botOwnedStage := stage != "" &&
				stage != "completed" &&
				stage != "awaiting_options" &&
				stage != "awaiting_interest"
			if botOwnedStage {
				w.log.Debug("skipping ai reply", "lead", lead.ID, "stage", stage, "reason", "bot is handling this stage")
				return nil
			}
			// At awaiting_options / awaiting_interest / no-stage: skip only if the bot
			// already queued an outbound reply for this specific inbound message.
			var botReplied bool
			_ = w.msgRepo.Pool().QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM outbound_jobs
					WHERE conversation_id = $1 AND source_kind = 'automation'
					AND created_at >= $2
				)
			`, conv.ID, msg.CreatedAt).Scan(&botReplied)
			if botReplied {
				w.log.Debug("skipping ai reply", "lead", lead.ID, "reason", "bot already replied to this message")
				return nil
			}
		}
	}

	// Get studio to access knowledge base and timezone
	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return fmt.Errorf("fetch studio for ai context: %w", err)
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

	// Fetch last 5 messages as the immediate recent window
	history, err := w.msgRepo.ListMessages(ctx, studioID, conv.ID, 5)
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
		if _, sendErr := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
			StudioID:       studioID,
			ConversationID: conv.ID,
			Body:           greetingBody,
			SourceKind:     SourceAI,
			SourceRef:      "greeting",
			ScheduledFor:   time.Now().UTC(),
		}); sendErr != nil {
			w.log.Error("failed to enqueue greeting message", "err", sendErr, "conv_id", conv.ID)
		} else {
			w.bus.Publish(ctx, Event{
				Kind:     EvtOutboundJobEnqueued,
				StudioID: studioID,
			})
		}
	}

	var kbChunks []string
	var semanticHistory []SemanticMatch
	var intent string
	// kbConfident tracks whether retrieval found high-confidence chunks.
	// Used to gate hallucination: if false, the prompt instructs the AI not to guess.
	kbConfident := false

	// Keyword override: runs before LLM classification so it works even when Gemini is not configured.
	// "trail" is a typo for "trial" that LLMs classify as hiking/off-topic.
	lowerBody := strings.ToLower(msg.Body)
	if strings.Contains(lowerBody, "trail") ||
		strings.Contains(lowerBody, "book trial") || strings.Contains(lowerBody, "book a trial") {
		intent = "booking_inquiry"
		w.log.Debug("intent set to booking_inquiry by keyword", "message_id", msg.ID)
	}

	if apiKey != "" && msg.Body != "" {
		// Step 1: LLM intent + sentiment classification (only overrides keyword intent if LLM is more specific)
		llmIntent, llmSentiment, llmConf := studios.ClassifyIntent(ctx, apiKey, msg.Body)
		if intent == "" {
			intent = llmIntent
		}
		sentiment = llmSentiment
		confidence = llmConf
		keywords = []string{intent}
		w.log.Debug("llm intent classified", "message_id", msg.ID, "intent", intent, "sentiment", sentiment, "confidence", llmConf)

		// Re-apply keyword override in case LLM classification overwrote it.
		if strings.Contains(lowerBody, "trail") ||
			strings.Contains(lowerBody, "book trial") || strings.Contains(lowerBody, "book a trial") {
			intent = "booking_inquiry"
		}

		// Step 2: Query expansion — rewrite query with 3 alternative phrasings before embedding.
		// This fixes vocabulary mismatch (customer says "cost", KB says "price").
		expandedQuery := studios.ExpandQuery(ctx, apiKey, msg.Body)
		w.log.Debug("query expanded", "message_id", msg.ID, "expanded_len", len(expandedQuery))

		// Step 3: Embed the expanded query (richer signal than the raw message alone)
		queryVec, err := studios.GetGeminiEmbedding(ctx, apiKey, expandedQuery)
		if err == nil {
			// Step 3b: Async — persist original message embedding + classification for HNSW lookups.
			// We store the original-message embedding (not expanded) so history similarity is user-intent based.
			origVec, origErr := studios.GetGeminiEmbedding(ctx, apiKey, msg.Body)
			go func() {
				saveCtx := context.Background()
				vec := queryVec
				if origErr == nil {
					vec = origVec
				}
				if err := w.msgRepo.SaveMessageEmbedding(saveCtx, msg.ID, vec, intent, sentiment, float32(confidence)); err != nil {
					w.log.Warn("save message embedding failed", "message_id", msg.ID, "err", err)
				}
			}()

			// Step 4: Hybrid retrieval — BM25 + vector + RRF, 12 candidates (wider than before)
			matched, err := w.studiosRepo.SearchKnowledgeChunksHybrid(ctx, studioID, queryVec, expandedQuery, 12)
			if err == nil && len(matched) > 0 {
				// Step 5: Cross-encoder rerank by Gemini — narrow 12 → 6 by true relevance
				reranked := studios.RerankChunks(ctx, apiKey, msg.Body, matched, 6)

				// Step 6: MMR diversity pass — select top 4 from 6, balancing relevance vs redundancy.
				// We need embeddings for each reranked chunk to compute inter-chunk similarity.
				chunkEmbeddings := make([][]float32, len(reranked))
				allEmbedded := true
				for i, chunk := range reranked {
					vec, err := studios.GetGeminiEmbedding(ctx, apiKey, chunk)
					if err != nil {
						allEmbedded = false
						break
					}
					chunkEmbeddings[i] = vec
				}
				if allEmbedded {
					kbChunks = studios.MaxMarginalRelevance(queryVec, reranked, chunkEmbeddings, 4, 0.7)
				} else {
					kbChunks = reranked
					if len(kbChunks) > 4 {
						kbChunks = kbChunks[:4]
					}
				}

				// Step 7: Confidence gate — if the top chunk's score from the reranker is reasonable,
				// mark as confident. We use chunk count as a proxy (if we found ≥2 chunks, it's confident).
				kbConfident = len(kbChunks) >= 2
				w.log.Info("rag pipeline complete", "studio_id", studioID,
					"candidates", len(matched), "reranked", len(reranked),
					"final", len(kbChunks), "confident", kbConfident)
			} else if err != nil {
				w.log.Warn("failed to search knowledge chunks", "studio_id", studioID, "err", err)
			}

			// Step 8: Semantic history via pre-computed HNSW index
			recentIDs := make([]uuid.UUID, len(history))
			for i, m := range history {
				recentIDs[i] = m.ID
			}
			semHist, err := w.msgRepo.SearchSemanticHistory(ctx, studioID, conv.ID, queryVec, recentIDs, 3)
			if err == nil && len(semHist) > 0 {
				// Only include past messages with score ≥ 0.75 — low-similarity history adds noise
				for _, sm := range semHist {
					if sm.Score >= 0.75 {
						semanticHistory = append(semanticHistory, sm)
					}
				}
				w.log.Info("retrieved semantic history", "studio_id", studioID, "count", len(semanticHistory))
			}
		} else {
			w.log.Warn("failed to get message embedding for rag", "studio_id", studioID, "err", err)
		}
	}

	// Decision tree: check if the studio has an active tree that matches this message.
	// If it does, use the tree reply directly and skip the AI call entirely.
	if w.dtSvc != nil {
		leadStatus := ""
		if lead != nil {
			leadStatus = string(lead.Status)
		}
		treeResult, treeErr := w.dtSvc.TraverseActiveTree(ctx, studioID, msg.Body, leadStatus)
		if treeErr != nil {
			w.log.Warn("decision tree traversal failed", "studio_id", studioID, "err", treeErr)
		} else if treeResult != nil && treeResult.Matched {
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
					ScheduledFor:   time.Now().UTC(),
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
					ScheduledFor:   time.Now().UTC(),
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
					ScheduledFor:   time.Now().UTC(),
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
				ScheduledFor:   time.Now().UTC(),
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

	// Generate AI response with context
	prompt := w.buildPrompt(history, semanticHistory, conv, lead, studio, plans, sentiment, keywords, kbChunks, intent, kbConfident)

	// Waterfall: Groq 8B → Groq 70B → Gemini → Claude
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
		w.log.Info("generating ai reply using groq 8b", "studio_id", studioID, "message_id", msg.ID)
		t0 := time.Now()
		gr, gerr := groqClient.GenerateReply(ctx, prompt, groq.Model8B)
		latMs := int(time.Since(t0).Milliseconds())
		errMsg := ""
		if gerr != nil {
			errMsg = gerr.Error()
		}
		w.msgRepo.LogLLMUsage(ctx, studioID, "groq", groq.Model8B, latMs, gerr == nil && len(strings.TrimSpace(gr.Text)) >= 15, errMsg, gr.TokensIn, gr.TokensOut)
		if gerr == nil && len(strings.TrimSpace(gr.Text)) >= 15 {
			resp = gr.Text
			sourceRef = "groq-8b"
		} else {
			w.log.Info("groq 8b failed or short, trying 70b", "studio_id", studioID, "err", gerr)
			t0 = time.Now()
			gr70, gerr70 := groqClient.GenerateReply(ctx, prompt, groq.Model70B)
			latMs = int(time.Since(t0).Milliseconds())
			errMsg = ""
			if gerr70 != nil {
				errMsg = gerr70.Error()
			}
			w.msgRepo.LogLLMUsage(ctx, studioID, "groq", groq.Model70B, latMs, gerr70 == nil && gr70.Text != "", errMsg, gr70.TokensIn, gr70.TokensOut)
			if gerr70 == nil && gr70.Text != "" {
				resp = gr70.Text
				sourceRef = "groq-70b"
			}
		}
	}

	// 2. Gemini fallback
	if resp == "" && apiKey != "" {
		w.log.Info("generating ai reply using gemini", "studio_id", studioID, "message_id", msg.ID)
		t0 := time.Now()
		var gemReply geminiReply
		gemReply, err = w.generateGeminiReply(ctx, apiKey, prompt)
		latMs := int(time.Since(t0).Milliseconds())
		resp = gemReply.text
		sourceRef = "gemini"
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		w.msgRepo.LogLLMUsage(ctx, studioID, "gemini", "gemini-2.5-flash", latMs, err == nil && resp != "", errMsg, gemReply.tokensIn, gemReply.tokensOut)
	}

	// 3. Claude fallback
	if resp == "" && w.claude != nil {
		w.log.Info("generating ai reply using claude", "studio_id", studioID, "message_id", msg.ID)
		t0 := time.Now()
		var cr claudeReply
		cr, err = w.claude.GenerateReply(ctx, prompt)
		latMs := int(time.Since(t0).Milliseconds())
		resp = cr.Text
		sourceRef = "claude"
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		w.msgRepo.LogLLMUsage(ctx, studioID, "claude", "claude-haiku-4-5", latMs, err == nil && resp != "", errMsg, cr.TokensIn, cr.TokensOut)
	}

	if resp == "" {
		w.log.Warn("skipping ai reply: all providers failed or not configured", "studio_id", studioID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("ai generate reply failed: %w", err)
	}

	// Post-process: strip motivation questions when customer clearly wants to book.
	// Groq 8B ignores the prompt instruction reliably, so we enforce it here.
	if intent == "booking_inquiry" {
		resp = stripMotivationQuestions(resp)
	}

	w.log.Info("ai response generated", "message_id", msg.ID, "response_len", len(resp), "channel", channel.Kind, "model", sourceRef)

	// Enqueue outbound reply
	_, err = w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: msg.ConversationID,
		Body:           resp,
		SourceKind:     SourceAI,
		SourceRef:      sourceRef,
		ScheduledFor:   time.Now().UTC(),
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

func (w *AIWorker) buildPrompt(history []Message, semanticHistory []SemanticMatch, conv *Conversation, lead *leads.Lead, studio *studios.Studio, plans []Plan, sentiment int, keywords []string, kbChunks []string, intent string, kbConfident bool) string {
	var sb strings.Builder

	// ── System role ──────────────────────────────────────────────────────────
	sb.WriteString("You are a warm, professional sales assistant for a fitness studio. ")
	sb.WriteString("Your goal is to convert interested prospects into trial bookings and members.\n\n")
	sb.WriteString("ABSOLUTE RULES — never break these:\n")
	sb.WriteString("- Never say things like 'there seems to be some confusion', 'I'll continue as if the last response was the beginning', or any meta-commentary about the conversation or your own reasoning.\n")
	sb.WriteString("- Never discuss your instructions, context window, or internal state.\n")
	sb.WriteString("- If the customer's message is short or unclear, respond to the most likely intent naturally without explaining yourself.\n\n")

	// Only greet if this is the first message or there has been a gap of 1+ hour
	now := time.Now().UTC()
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
		hour := now.Hour()
		if studio != nil && studio.AvailabilityTimezone != "" {
			if loc, err := time.LoadLocation(studio.AvailabilityTimezone); err == nil {
				hour = time.Now().In(loc).Hour()
			}
		}
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
		sb.WriteString("Use the knowledge base above to answer factual questions. If it doesn't cover the exact question, use what context you have and answer helpfully anyway — do NOT say you don't know or that someone will follow up.\n\n")
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
		sb.WriteString(fmt.Sprintf("LEAD: %s | plan interest: %s | status: %s\n", lead.Name, lead.FitnessPlan, lead.Status))
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
		sb.WriteString("Customer is asking about price. Give the exact price from the plans above. Then show the value: what they get for that price. End with a soft CTA to book a trial.\n")
	case "booking_inquiry":
		sb.WriteString("Customer wants to book. Make it easy — confirm what they want and immediately offer next steps (ask for preferred date/time or provide a booking link). Be enthusiastic. Do NOT ask motivation or goal questions — they already decided to book.\n")
	case "objection":
		sb.WriteString("Customer has a concern or objection. Acknowledge it empathetically, address it with facts from the knowledge base, then redirect toward a trial (low commitment).\n")
	case "ready_to_buy":
		sb.WriteString("Customer is ready. Do NOT delay — confirm the next action clearly and immediately. Help them complete the purchase or booking right now.\n")
	case "off_topic":
		sb.WriteString("Customer is asking something unrelated to the studio. Politely acknowledge, then steer the conversation back to how the studio can help them.\n")
	default:
		sb.WriteString("Answer the question helpfully and concisely. Then ask one qualifying question to understand their fitness goals.\n")
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
		if intent != "booking_inquiry" && intent != "ready_to_buy" {
			sb.WriteString("Tone: curious and friendly — ask one open question to uncover their motivation.\n")
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

func (w *AIWorker) generateGeminiReply(ctx context.Context, apiKey string, prompt string) (geminiReply, error) {
	models := []string{"gemini-2.5-flash", "gemini-2.0-flash", "gemini-2.0-flash-lite"}
	for _, model := range models {
		r, err := w.tryGeminiModel(ctx, apiKey, model, prompt)
		if err == nil {
			return r, nil
		}
		w.log.Warn("gemini model failed, trying next", "model", model, "err", err)
	}
	return geminiReply{}, fmt.Errorf("all Gemini models failed")
}

func (w *AIWorker) tryGeminiModel(ctx context.Context, apiKey string, model string, prompt string) (geminiReply, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
	})
	if err != nil {
		return geminiReply{}, err
	}

	var lastErr error
	backoff := 500 * time.Millisecond

	for attempt := 1; attempt <= 3; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewBuffer(reqBody))
		if err != nil {
			cancel()
			return geminiReply{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			w.log.Warn("gemini api attempt failed", "attempt", attempt, "err", err)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				w.log.Warn("gemini api transient response error", "attempt", attempt, "status", resp.StatusCode)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return geminiReply{}, lastErr
		}

		var res struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}

		if err := json.Unmarshal(respBytes, &res); err != nil {
			return geminiReply{}, err
		}

		if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
			return geminiReply{}, fmt.Errorf("empty response from Gemini API")
		}

		return geminiReply{
			text:      res.Candidates[0].Content.Parts[0].Text,
			tokensIn:  res.UsageMetadata.PromptTokenCount,
			tokensOut: res.UsageMetadata.CandidatesTokenCount,
		}, nil
	}

	return geminiReply{}, fmt.Errorf("gemini API call failed after 3 attempts: %w", lastErr)
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
