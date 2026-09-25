package messaging

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/studios"
)

// ErrNoProviderConfigured is safe to show directly to an admin — it's
// actionable ("go set up a provider"), not an internal-error leak.
var ErrNoProviderConfigured = errors.New("no AI provider configured or all providers failed")

// TestChatTurn is one message in a test-chat session, as sent by the client.
// The client maintains the running transcript itself — nothing here is
// persisted server-side.
type TestChatTurn struct {
	Role string `json:"role"` // "user" | "assistant"
	Text string `json:"text"`
}

type TestChatRequest struct {
	Message string         `json:"message"`
	History []TestChatTurn `json:"history"`
	// Timezone is the admin's own browser/system IANA timezone (e.g.
	// "Asia/Kolkata"), sent automatically by the client on every request —
	// not typed by the admin. buildPrompt's greeting instruction uses it
	// instead of the studio's AvailabilityTimezone, so the greeting an admin
	// sees in Test Chat matches their own current time of day. There's no
	// real recipient in Test Chat to derive a timezone from a phone number,
	// so this is the client's clock, not a phone lookup. Falls back to the
	// studio's timezone if empty or unrecognized.
	Timezone string `json:"timezone,omitempty"`
}

type TestChatResponse struct {
	Reply string `json:"reply"`
	// Sources lists the uploaded knowledge-base document names (deduplicated,
	// in relevance order) that actually contributed to Reply — lets an admin
	// verify retrieval is pulling from the right document before enabling
	// live AI replies. Empty when no knowledge-base chunk was used (e.g. a
	// greeting, or a question the KB has no relevant content for).
	Sources []string `json:"sources,omitempty"`
}

// TestChat runs the studio's real knowledge-base + LLM pipeline for a
// one-off question, with no database writes and no dependency on a real
// conversation/lead — used by the Knowledge Base admin page's test-chat
// drawer so an admin can validate retrieval + tone before enabling live AI
// replies. Deliberately reuses the same building blocks as the real
// per-message pipeline (studios.ClassifyIntent/ExpandQuery/RerankChunks,
// SearchKnowledgeChunksHybrid, SearchStyleExamples, buildPrompt,
// llmWaterfall) rather than handleMessage, which is tightly coupled to real
// Message/Conversation/Lead records and has side effects (DB writes, job
// enqueuing) that would be wrong here.
func (w *AIWorker) TestChat(ctx context.Context, studioID uuid.UUID, req TestChatRequest) (string, []string, error) {
	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return "", nil, fmt.Errorf("fetch studio: %w", err)
	}

	apiKey := studio.GeminiAPIKey
	if apiKey == "" {
		if pk, err := w.studiosRepo.GetPlatformSetting(ctx, "gemini_api_key"); err == nil && pk != "" {
			apiKey = pk
		}
	}

	intent := "general_question"
	sentiment := 0
	var kbChunks []string
	var sources []string
	var styleExamples []StyleExample

	geminiModel := ""
	if models, mErr := w.llmRepo.EnabledModelsForStudio(ctx, studioID, llm.ProviderGemini); mErr == nil && len(models) > 0 {
		geminiModel = models[0]
	}

	if apiKey != "" && req.Message != "" {
		i, s, _ := studios.ClassifyIntent(ctx, apiKey, geminiModel, req.Message)
		intent, sentiment = i, s
		expandedQuery := studios.ExpandQuery(ctx, apiKey, geminiModel, req.Message)

		if queryVec, err := w.embeddings.EmbedQuery(ctx, expandedQuery); err == nil {
			if matched, err := w.studiosRepo.SearchKnowledgeChunksHybrid(ctx, studioID, queryVec, expandedQuery, "all", 8); err == nil && len(matched) > 0 {
				contents := make([]string, len(matched))
				sourceByContent := make(map[string]string, len(matched))
				for idx, m := range matched {
					contents[idx] = m.Content
					sourceByContent[m.Content] = m.SourceName
				}
				kbChunks = studios.RerankChunks(ctx, apiKey, geminiModel, req.Message, contents, 4)

				seenSource := make(map[string]bool)
				for _, c := range kbChunks {
					name := sourceByContent[c]
					if name == "" || seenSource[name] {
						continue
					}
					seenSource[name] = true
					sources = append(sources, name)
				}
			}
			if examples, err := w.msgRepo.SearchStyleExamples(ctx, studioID, queryVec, 3); err == nil {
				styleExamples = examples
			}
		}
	}

	plans, err := w.msgRepo.ListActivePlans(ctx, studioID)
	if err != nil {
		plans = []Plan{}
	}

	// Convert the client-supplied prior turns into the same []Message shape
	// buildPrompt already understands (it only reads Direction, Body, and —
	// for its greeting-vs-no-greeting decision — the last outbound message's
	// SentAt). No DB row involved, purely in-memory. SentAt must be non-zero
	// and recent: buildPrompt treats a zero SentAt as "first contact ever"
	// and re-greets on every turn otherwise.
	now := time.Now()
	history := make([]Message, 0, len(req.History)+1)
	for _, t := range req.History {
		dir := DirectionOutbound
		if t.Role == "user" {
			dir = DirectionInbound
		}
		history = append(history, Message{Direction: dir, Body: t.Text, SentAt: now})
	}
	// The current question itself must be the last entry — buildPrompt's
	// "RECENT CONVERSATION" section is what the model actually responds to;
	// without this, the model sees the prior turns (or nothing, on the
	// first message) and free-associates a generic reply instead of
	// answering the question just asked.
	history = append(history, Message{Direction: DirectionInbound, Body: req.Message, SentAt: now})

	prompt, expectedGreeting := w.buildPrompt(ctx, history, nil, styleExamples, nil, nil, studio, plans, sentiment, nil, kbChunks, intent, len(kbChunks) >= 2, "", req.Timezone)

	reply, _ := llmWaterfall(ctx, w.studiosRepo, w.llmRepo, w.msgRepo, w.claude, w.claudeAPIURL, w.log, studioID, studio, prompt)
	if reply == "" {
		return "", nil, ErrNoProviderConfigured
	}
	reply = enforceGreeting(reply, expectedGreeting)
	return reply, sources, nil
}

// TestChatHandler godoc
//
//	@Summary		Test the knowledge-base + AI reply pipeline
//	@Description	Runs a one-off question through the studio's real knowledge-base retrieval and LLM reply pipeline, without creating any conversation, message, or lead record.
//	@Tags			Knowledge Base
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string				true	"Studio ID (UUID)"
//	@Param			body		body		TestChatRequest		true	"Test message and prior turns in this session"
//	@Success		200			{object}	TestChatResponse
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		422			{object}	httpx.ErrorResponse	"message is required"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/knowledge-base/test-chat [post]
func (w *AIWorker) TestChatHandler(rw http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(rw, r)
	if !ok {
		return
	}
	var req TestChatRequest
	if !httpx.DecodeJSON(rw, r, &req) {
		return
	}
	if req.Message == "" {
		httpx.WriteValidationError(rw, map[string]string{"message": "required"})
		return
	}
	reply, sources, err := w.TestChat(r.Context(), studioID, req)
	if err != nil {
		if errors.Is(err, ErrNoProviderConfigured) {
			httpx.WriteError(rw, http.StatusUnprocessableEntity, "no_provider", err.Error())
			return
		}
		w.log.Error("test chat failed", "studio_id", studioID, "err", err)
		httpx.WriteError(rw, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(rw, http.StatusOK, TestChatResponse{Reply: reply, Sources: sources})
}
