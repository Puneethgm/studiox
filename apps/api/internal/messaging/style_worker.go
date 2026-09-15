package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/studios"
)

const (
	// stylePollInterval is intentionally much longer than the outbox workers'
	// 5s polls — rebuilding a style profile isn't latency-sensitive, and a
	// missed tick just retries next cycle with no data loss.
	stylePollInterval = 30 * time.Minute
	// styleRefreshThreshold is how many NEW staff replies (since the last
	// build) a studio needs before its profile is worth rebuilding.
	styleRefreshThreshold = 20
	// styleSampleSize caps how many of a studio's most recent staff replies
	// are fed to the LLM for one profile build.
	styleSampleSize = 100
	// backfillEmbedCap bounds how many historical staff replies get embedded
	// in one go right after a WhatsApp/Telegram Web backfill finishes — a
	// studio switching over can have years of history; only the most recent
	// slice is worth the embedding-API cost, since that's plenty to seed
	// both retrieval and the first style-profile build.
	backfillEmbedCap = 300
	// retryEmbedCap bounds the recurring per-tick retry sweep (see
	// retryUnembeddedStaffReplies) — smaller than backfillEmbedCap since
	// this runs every cycle forever, not once right after an import, and in
	// the common case (a key was configured from the start) it finds nothing
	// and does zero API calls.
	retryEmbedCap = 50
)

// StyleWorker periodically distills each studio's own staff-authored replies
// (source_kind='studio_user') into a short, human-readable "communication
// style" writeup — tone, phrasing, emoji use, how they handle pricing/
// objections — stored on the studio and shown/editable on the Knowledge Base
// page. This is the studio-visible complement to SearchStyleExamples'
// per-reply few-shot retrieval: a stable style anchor injected into every AI
// prompt, distilled once per refresh instead of retrieved fresh each time.
type StyleWorker struct {
	studiosRepo *studios.Repo
	msgRepo     *Repo
	claude      *claude.Client
	log         *slog.Logger
}

func NewStyleWorker(studiosRepo *studios.Repo, msgRepo *Repo, claudeClient *claude.Client, logger *slog.Logger) *StyleWorker {
	return &StyleWorker{studiosRepo: studiosRepo, msgRepo: msgRepo, claude: claudeClient, log: logger}
}

func (w *StyleWorker) Run(ctx context.Context) {
	if w.studiosRepo == nil || w.msgRepo == nil {
		w.log.Warn("style worker disabled — missing deps")
		return
	}
	w.log.Info("style worker started", "poll_interval", stylePollInterval, "threshold", styleRefreshThreshold)

	// One-time reconciliation on startup: staff_reply_count_total only grows
	// from messages sent after this feature shipped (or from a backfill
	// completing while the server is up to hear EvtWAWebBackfillDone). Any
	// studio with staff-reply history that predates both of those — which is
	// every studio that existed before today — would otherwise sit at
	// counter=0 forever and never cross styleRefreshThreshold, no matter how
	// much real history it has. This sweep makes existing history count too.
	w.reconcileAllStudios(ctx)

	t := time.NewTicker(stylePollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("style worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// ListenForNewReplies subscribes to every message-sent event platform-wide
// (uuid.Nil is the bus's "global" subscription — see events.go) so it learns
// from a staff reply no matter which path created it: sent through this
// platform's Inbox (worker.go's outbound dispatch), typed directly on a
// linked WhatsApp/Telegram account ("fromMe" live messages, which never go
// through that dispatch path), or any future channel added later — one
// subscriber covers all of them instead of hooking each send site by hand.
// It also listens for EvtWAWebBackfillDone to catch up on a studio's
// existing chat history the moment a QR-linked account finishes importing,
// rather than waiting for new replies to trickle in one at a time.
func (w *StyleWorker) ListenForNewReplies(ctx context.Context, bus Bus) {
	ch, unsub := bus.Subscribe(uuid.Nil)
	defer unsub()
	w.log.Info("style worker listening for staff replies")
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			switch evt.Kind {
			case EvtMessageSent:
				if evt.MessageID == nil {
					continue
				}
				go w.learnFromMessageIfStaffReply(context.Background(), evt.StudioID, *evt.MessageID)
			case EvtWAWebBackfillDone:
				go w.catchUpBackfilledReplies(context.Background(), evt.StudioID)
			}
		}
	}
}

// learnFromMessageIfStaffReply embeds a just-sent message and bumps the
// studio's staff-reply counter, but only if a human actually typed it
// (source_kind='studio_user') — AI and automation sends are excluded so the
// AI never reinforces its own generic phrasing as "the studio's voice."
func (w *StyleWorker) learnFromMessageIfStaffReply(ctx context.Context, studioID, messageID uuid.UUID) {
	msg, err := w.msgRepo.GetMessageByID(ctx, studioID, messageID)
	if err != nil || msg == nil || msg.SourceKind != SourceStudioUser {
		return
	}
	if err := w.studiosRepo.IncrementStaffReplyCount(ctx, studioID); err != nil {
		w.log.Warn("increment staff reply count failed", "studio_id", studioID, "err", err)
	}
	w.embedStaffReply(ctx, studioID, messageID, msg.Body)
}

// catchUpBackfilledReplies reconciles staff_reply_count_total against the
// real count after a backfill import (which inserts rows directly via
// InsertMessageBackfill, bypassing the live per-send hook above), then
// embeds up to backfillEmbedCap of the newly-visible staff replies so
// retrieval and the next style-profile build can use a studio's existing
// history immediately instead of only replies sent from here on.
func (w *StyleWorker) catchUpBackfilledReplies(ctx context.Context, studioID uuid.UUID) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	total, err := w.msgRepo.CountStaffReplies(ctx, studioID)
	if err != nil {
		w.log.Warn("backfill catch-up: count staff replies failed", "studio_id", studioID, "err", err)
		return
	}
	if err := w.studiosRepo.SetStaffReplyCountTotal(ctx, studioID, total); err != nil {
		w.log.Warn("backfill catch-up: set staff reply count failed", "studio_id", studioID, "err", err)
	}

	w.embedUnembeddedStaffReplies(ctx, studioID, backfillEmbedCap)
}

// retryUnembeddedStaffReplies is the recurring safety net for "the LLM key
// wasn't configured yet when a message was first seen." embedStaffReply
// silently skips embedding when no Gemini key is available at that moment
// (studio's own or the platform-wide fallback), and — unlike a failed API
// call — there was previously nothing that ever came back to retry it. This
// runs every tick, for every studio: in the common case (a key already
// configured) ListUnembeddedStaffReplies finds nothing and it costs one
// cheap indexed query per studio, no API calls. Only a studio that actually
// has a backlog (key added after some replies were already missed) does
// real embedding work here, capped at retryEmbedCap per cycle so a large
// backlog drains gradually instead of bursting the embedding API.
func (w *StyleWorker) retryUnembeddedStaffReplies(ctx context.Context) {
	list, err := w.studiosRepo.List(ctx)
	if err != nil {
		w.log.Error("style worker: list studios for embedding retry failed", "err", err)
		return
	}
	for _, s := range list {
		studioCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		w.embedUnembeddedStaffReplies(studioCtx, s.ID, retryEmbedCap)
		cancel()
	}
}

// embedUnembeddedStaffReplies embeds up to `limit` of a studio's staff
// replies that don't have an embedding yet, rate-limited the same way the
// Knowledge Base sync job is. Shared by the one-shot backfill catch-up and
// the recurring retry sweep — the only difference between the two callers
// is the cap and how often each runs. Checks for a usable API key once,
// up front, rather than per message — a studio with no key configured yet
// should cost one cheap lookup here, not `limit` wasted round-trips.
func (w *StyleWorker) embedUnembeddedStaffReplies(ctx context.Context, studioID uuid.UUID, limit int) {
	apiKey, err := w.resolveGeminiAPIKey(ctx, studioID)
	if err != nil {
		w.log.Warn("style learning: fetch studio failed", "studio_id", studioID, "err", err)
		return
	}
	if apiKey == "" {
		return
	}

	unembedded, err := w.msgRepo.ListUnembeddedStaffReplies(ctx, studioID, limit)
	if err != nil {
		w.log.Warn("list unembedded staff replies failed", "studio_id", studioID, "err", err)
		return
	}
	if len(unembedded) == 0 {
		return
	}
	w.log.Info("embedding staff replies", "studio_id", studioID, "count", len(unembedded))
	for _, msg := range unembedded {
		vec, err := studios.GetGeminiEmbedding(ctx, apiKey, msg.Body)
		if err != nil {
			w.log.Warn("style learning: embed staff reply failed", "message_id", msg.ID, "err", err)
			continue
		}
		if err := w.msgRepo.SaveMessageEmbedding(ctx, msg.ID, vec, "", 0, 0); err != nil {
			w.log.Warn("style learning: save staff reply embedding failed", "message_id", msg.ID, "err", err)
		}
		time.Sleep(100 * time.Millisecond) // same embedding-API rate-limit as the Knowledge Base sync job
	}
}

// resolveGeminiAPIKey returns a studio's own Gemini key, falling back to the
// platform-wide one (Superadmin → Platform Settings) if the studio hasn't
// set its own — same fallback order used everywhere else in this file.
func (w *StyleWorker) resolveGeminiAPIKey(ctx context.Context, studioID uuid.UUID) (string, error) {
	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return "", err
	}
	if studio.GeminiAPIKey != "" {
		return studio.GeminiAPIKey, nil
	}
	pk, _ := w.studiosRepo.GetPlatformSetting(ctx, "gemini_api_key")
	return pk, nil
}

// embedStaffReply resolves a Gemini API key and saves an embedding for a
// single staff-authored message — used by the live per-send path, where
// resolving the key inline for one message is cheap.
func (w *StyleWorker) embedStaffReply(ctx context.Context, studioID, messageID uuid.UUID, body string) {
	apiKey, err := w.resolveGeminiAPIKey(ctx, studioID)
	if err != nil {
		w.log.Warn("style learning: fetch studio failed", "studio_id", studioID, "err", err)
		return
	}
	if apiKey == "" {
		return
	}
	vec, err := studios.GetGeminiEmbedding(ctx, apiKey, body)
	if err != nil {
		w.log.Warn("style learning: embed staff reply failed", "message_id", messageID, "err", err)
		return
	}
	if err := w.msgRepo.SaveMessageEmbedding(ctx, messageID, vec, "", 0, 0); err != nil {
		w.log.Warn("style learning: save staff reply embedding failed", "message_id", messageID, "err", err)
	}
}

// reconcileAllStudios runs catchUpBackfilledReplies for every studio once,
// at worker startup — see the comment on Run for why this is needed on top
// of the event-driven catch-up.
func (w *StyleWorker) reconcileAllStudios(ctx context.Context) {
	list, err := w.studiosRepo.List(ctx)
	if err != nil {
		w.log.Error("style worker: list studios for reconciliation failed", "err", err)
		return
	}
	for _, s := range list {
		w.catchUpBackfilledReplies(ctx, s.ID)
	}
	w.log.Info("style worker: startup reconciliation complete", "studios", len(list))
}

func (w *StyleWorker) tick(ctx context.Context) {
	w.retryUnembeddedStaffReplies(ctx)

	studioIDs, err := w.studiosRepo.ListStudiosNeedingStyleRefresh(ctx, styleRefreshThreshold)
	if err != nil {
		w.log.Error("list studios needing style refresh", "err", err)
		return
	}
	for _, id := range studioIDs {
		if err := w.rebuildProfile(ctx, id); err != nil {
			w.log.Warn("rebuild style profile failed", "studio_id", id, "err", err)
		}
	}
}

func (w *StyleWorker) rebuildProfile(ctx context.Context, studioID uuid.UUID) error {
	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return fmt.Errorf("load studio: %w", err)
	}
	examples, err := w.msgRepo.ListRecentStaffReplies(ctx, studioID, styleSampleSize)
	if err != nil {
		return fmt.Errorf("list recent staff replies: %w", err)
	}
	if len(examples) == 0 {
		return nil
	}

	var doc strings.Builder
	for _, ex := range examples {
		if ex.CustomerMessage != "" {
			fmt.Fprintf(&doc, "Customer: %s\n", ex.CustomerMessage)
		}
		fmt.Fprintf(&doc, "Staff: %s\n\n", ex.StudioReply)
	}

	prompt := "You are analyzing real customer-service message transcripts from a single " +
		"fitness studio, to describe HOW this studio's staff talk to customers — not what " +
		"the business does. Read the transcripts below and write a short (120-180 word), " +
		"plain-English profile covering: overall tone/formality, phrases or greetings they " +
		"reuse often, emoji usage (if any), and how they typically handle pricing questions " +
		"or hesitant customers. Write it as direct instructions to someone else who will " +
		"impersonate this studio's voice (e.g. \"Keep replies short and upbeat...\", not " +
		"\"The studio is short and upbeat\"). Output only the profile text, no preamble.\n\n" +
		"TRANSCRIPTS:\n" + doc.String()

	// Same Groq → Gemini → Claude waterfall the AI reply pipeline uses — this
	// worker can't assume Claude is configured, since it's a single
	// platform-wide key while Groq/Gemini keys are set per studio.
	replyText, sourceRef := llmWaterfall(ctx, http.DefaultClient, w.studiosRepo, w.msgRepo, w.claude, w.log, studioID, studio, prompt)
	profile := strings.TrimSpace(replyText)
	if profile == "" {
		return fmt.Errorf("no LLM provider configured or all providers failed")
	}
	w.log.Info("style profile generated", "studio_id", studioID, "provider", sourceRef)
	if err := w.studiosRepo.SetCommunicationStyleProfile(ctx, studioID, profile); err != nil {
		return fmt.Errorf("save style profile: %w", err)
	}
	w.log.Info("rebuilt communication style profile", "studio_id", studioID, "sample_size", len(examples))
	return nil
}
