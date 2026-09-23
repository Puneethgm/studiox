package messaging

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/messaging/channels"
)

// OutboundWorker drains the outbound_jobs queue and dispatches via the
// channel adapter. Single in-process worker for now; multiple replicas would
// race-safely thanks to FOR UPDATE SKIP LOCKED in ClaimOutboundBatch.
type OutboundWorker struct {
	repo      *Repo
	bus       Bus
	whatsapp  channels.Sender
	messenger channels.Sender
	instagram channels.Sender
	twilio    channels.Sender
	x         channels.Sender
	telegram  channels.Sender
	log       *slog.Logger

	waPaceMu   sync.Mutex
	waLastSent map[uuid.UUID]time.Time
}

const (
	workerPollInterval = 2 * time.Second
	workerBatchSize    = 10
	maxAttempts        = 6

	// defaultWhatsAppSendSpacing throttles consecutive WhatsApp sends on the
	// same channel so bulk sends (sheet import, fresh connect, etc.) don't
	// fire back-to-back and risk the number getting flagged/banned by
	// WhatsApp. A studio can override this via the Settings UI
	// (studios.whatsapp_send_spacing_seconds); this is only the fallback
	// used if that lookup fails.
	defaultWhatsAppSendSpacing = 20 * time.Second
)

func NewOutboundWorker(repo *Repo, bus Bus, whatsapp, messenger, instagram, twilio, x, telegram channels.Sender, log *slog.Logger) *OutboundWorker {
	return &OutboundWorker{
		repo:       repo,
		bus:        bus,
		whatsapp:   whatsapp,
		messenger:  messenger,
		instagram:  instagram,
		twilio:     twilio,
		x:          x,
		telegram:   telegram,
		log:        log,
		waLastSent: make(map[uuid.UUID]time.Time),
	}
}

// paceWhatsAppSend blocks, if needed, so that consecutive sends on the same
// WhatsApp channel are spaced at least `spacing` apart. Pacing is tracked per
// channel so independent numbers aren't throttled by each other.
func (w *OutboundWorker) paceWhatsAppSend(ctx context.Context, channelID uuid.UUID, spacing time.Duration) {
	w.waPaceMu.Lock()
	last, ok := w.waLastSent[channelID]
	wait := time.Duration(0)
	now := time.Now()
	if ok {
		elapsed := now.Sub(last)
		if elapsed < spacing {
			wait = spacing - elapsed
		}
	}
	w.waLastSent[channelID] = now.Add(wait)
	w.waPaceMu.Unlock()

	if wait <= 0 {
		return
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (w *OutboundWorker) Run(ctx context.Context) {
	w.log.Info("outbound worker started", "poll", workerPollInterval, "batch", workerBatchSize)
	t := time.NewTicker(workerPollInterval)
	defer t.Stop()

	eventsCh, unsub := w.bus.Subscribe(uuid.Nil)
	defer unsub()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("outbound worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		case evt, ok := <-eventsCh:
			if !ok {
				return
			}
			if evt.Kind == EvtOutboundJobEnqueued {
				w.tick(ctx)
			}
		}
	}
}

func (w *OutboundWorker) tick(ctx context.Context) {
	jobs, err := w.repo.ClaimOutboundBatch(ctx, workerBatchSize)
	if err != nil {
		w.log.Error("claim outbound batch", "err", err)
		return
	}
	for _, j := range jobs {
		w.dispatch(ctx, j)
	}
}

// formatMarkdownTablesAsPlainText rewrites any markdown pipe-table in body
// into a plain, line-per-row format. buildPrompt asks the model for a
// markdown table on schedule/timetable questions (see the "format that part
// of your answer as a markdown table" instruction) because Test Chat renders
// it as a real HTML table — but that's the only surface that does. Real
// channels (WhatsApp, Telegram, SMS, Instagram, Messenger) have no table
// rendering, so without this the customer would receive literal `|` and
// `---` characters. This keeps the same "visually grouped by line" goal,
// just with readable separators instead of unrendered markdown syntax.
func formatMarkdownTablesAsPlainText(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))

	isTableRow := func(line string) bool {
		t := strings.TrimSpace(line)
		return len(t) > 1 && strings.HasPrefix(t, "|") && strings.HasSuffix(t, "|")
	}
	isSeparatorRow := func(line string) bool {
		t := strings.TrimSpace(line)
		if !strings.Contains(t, "-") {
			return false
		}
		for _, r := range t {
			switch r {
			case '|', '-', ':', ' ':
			default:
				return false
			}
		}
		return true
	}
	splitRow := func(line string) []string {
		t := strings.TrimSpace(line)
		t = strings.TrimPrefix(t, "|")
		t = strings.TrimSuffix(t, "|")
		parts := strings.Split(t, "|")
		cells := make([]string, len(parts))
		for i, p := range parts {
			cells[i] = strings.TrimSpace(p)
		}
		return cells
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if i+1 < len(lines) && isTableRow(line) && isSeparatorRow(lines[i+1]) {
			headers := splitRow(line)
			j := i + 2
			var rows [][]string
			for j < len(lines) && isTableRow(lines[j]) {
				rows = append(rows, splitRow(lines[j]))
				j++
			}
			if len(rows) > 0 {
				out = append(out, strings.Join(headers, " / ")+":")
				for _, row := range rows {
					out = append(out, strings.Join(row, " — "))
				}
				i = j - 1
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func (w *OutboundWorker) dispatch(ctx context.Context, j OutboundJob) {
	// 1. Resolve the conversation → channel + recipient.
	conv, err := w.repo.GetConversation(ctx, j.StudioID, j.ConversationID)
	if err != nil {
		w.failJob(ctx, j, "conversation lookup: "+err.Error(), false)
		return
	}
	channel, err := w.repo.GetChannelByID(ctx, j.StudioID, conv.ChannelAccountID)
	if err != nil {
		w.failJob(ctx, j, "channel lookup: "+err.Error(), true) // dead — channel deleted
		return
	}

	// Resolve template variables for the message body
	var studioName string
	err = w.repo.Pool().QueryRow(ctx, "SELECT name FROM studios WHERE id = $1", j.StudioID).Scan(&studioName)
	if err != nil {
		w.log.Error("failed to fetch studio name for template replacement", "err", err)
	}

	contactFirstName := ""
	campaignName := ""
	if conv.LeadID != nil {
		err = w.repo.Pool().QueryRow(ctx, `
			SELECT COALESCE(NULLIF(l.first_name, ''), SPLIT_PART(l.name, ' ', 1)), COALESCE(c.name, '')
			FROM leads l
			LEFT JOIN campaigns c ON l.campaign_id = c.id
			WHERE l.id = $1
		`, *conv.LeadID).Scan(&contactFirstName, &campaignName)
		if err != nil {
			w.log.Error("failed to fetch lead/campaign details for template replacement", "err", err)
		}
	}

	if contactFirstName == "" {
		if conv.ContactDisplayName != "" {
			contactFirstName = strings.Split(conv.ContactDisplayName, " ")[0]
		} else {
			contactFirstName = "there"
		}
	}
	// Sanity check: if the name still looks like a raw chat ID, fall back to "there"
	if strings.Contains(contactFirstName, "@") {
		contactFirstName = "there"
	}

	// Replace placeholders in the body
	j.Body = strings.ReplaceAll(j.Body, "{{contact.first_name}}", contactFirstName)
	j.Body = strings.ReplaceAll(j.Body, "{{studio.name}}", studioName)
	j.Body = strings.ReplaceAll(j.Body, "{{campaign.name}}", campaignName)
	j.Body = formatMarkdownTablesAsPlainText(j.Body)
	// In local/dev mode, allow error status channels for testing.
	isLocalDev := os.Getenv("API_ENV") == "local"
	if channel.Status != StatusActive {
		// Try to find an active channel of the same kind for this studio.
		activeChannel, err := w.repo.GetActiveChannelByKind(ctx, j.StudioID, channel.Kind)
		if err == nil && activeChannel.Status == StatusActive {
			// Found an active channel of the same kind; use that instead.
			channel = activeChannel
		} else if !isLocalDev {
			w.failJob(ctx, j, "no active channel: "+string(channel.Status), false)
			return
		}
		// In local mode, continue even if no active channel found.
	}

	// Daily automated-send cap — automation/AI-sourced WhatsApp sends are
	// counted and blocked, which also covers Manual Actions jobs (Service.
	// CreateJob tags those SourceAutomation for exactly this reason). A live
	// reply typed directly in a conversation (SourceStudioUser, EnqueueReply)
	// always goes through uncapped. This exists to keep unverified WhatsApp
	// numbers under Meta's low messaging-limit tier from getting
	// flagged/banned by unattended bulk sending. A studio's cap is 0 =
	// unlimited. Lookup failures fail open (send proceeds) rather than
	// blocking messaging on a transient DB hiccup, matching the existing
	// send-spacing fallback convention.
	if (channel.Kind == KindWhatsAppMeta || channel.Kind == KindWhatsAppWeb) &&
		(j.SourceKind == SourceAutomation || j.SourceKind == SourceAI) {
		if limit, err := w.repo.GetWhatsAppDailyMessageLimit(ctx, j.StudioID); err == nil && limit > 0 {
			if sentToday, err := w.repo.CountAutomatedWhatsAppSentToday(ctx, j.StudioID); err == nil && sentToday >= limit {
				w.failJob(ctx, j, "daily_limit_exceeded", true)
				return
			}
		}
	}

	var sender channels.Sender
	switch channel.Kind {
	case KindWhatsAppMeta:
		if isLocalDev && channel.AccessToken == "" {
			w.log.Info("test mode: using mock sender for WA", "job_id", j.ID)
			sender = &testSender{}
		} else {
			sender = w.whatsapp
		}
	case KindInstagramMeta:
		if isLocalDev && channel.AccessToken == "" {
			w.log.Info("test mode: using mock sender for Meta Messaging", "job_id", j.ID)
			sender = &testSender{}
		} else {
			sender = w.instagram
		}
	case KindMessengerMeta:
		if isLocalDev && channel.AccessToken == "" {
			w.log.Info("test mode: using mock sender for Meta Messaging", "job_id", j.ID)
			sender = &testSender{}
		} else {
			sender = w.messenger
		}
	case KindSMS:
		if isLocalDev && channel.AccessToken == "test:test" {
			sender = &testSender{}
		} else {
			sender = w.twilio
		}
	case KindXDM:
		sender = w.x
	case KindTelegram:
		sender = w.telegram
	case KindWhatsAppWeb:
		sender = &waWebSender{studioID: j.StudioID}
	case KindTelegramMTProto:
		sender = &tgWebSender{studioID: j.StudioID}
	default:
		w.failJob(ctx, j, "no sender for channel kind: "+string(channel.Kind), true)
		return
	}

	// 2. Send via the channel adapter.
	// Convert domain Attachment → channels.Attachment for the sender.
	var chAtts []channels.Attachment
	for _, a := range j.Attachments {
		chAtts = append(chAtts, channels.Attachment{Type: a.Type, URL: a.URL, Name: a.Name})
	}
	if channel.Kind == KindWhatsAppMeta || channel.Kind == KindWhatsAppWeb {
		spacing := defaultWhatsAppSendSpacing
		if secs, err := w.repo.GetWhatsAppSendSpacing(ctx, j.StudioID); err == nil {
			spacing = time.Duration(secs) * time.Second
		}
		w.paceWhatsAppSend(ctx, channel.ID, spacing)
	}
	res, err := sender.SendText(ctx, channel.AccessToken, channel.ExternalID, conv.ContactValue, j.Body, chAtts)
	if err != nil {
		// Credential errors are terminal for this job; mark channel error too.
		if errors.Is(err, channels.ErrInvalidCredentials) {
			_ = w.repo.MarkChannelError(ctx, channel.ID, err.Error())
			w.failJob(ctx, j, "credentials: "+err.Error(), true)
			return
		}
		w.failJob(ctx, j, err.Error(), j.Attempts+1 >= maxAttempts)
		return
	}

	// 3. Persist the outbound message + bump conversation snapshot. Tx so the
	//    conversation's last_message_* stays consistent with the message row.
	tx, err := w.repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		w.failJob(ctx, j, "tx begin: "+err.Error(), false)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	msg, err := w.repo.InsertMessage(ctx, tx, CreateMessageInput{
		ConversationID: conv.ID,
		StudioID:       j.StudioID,
		Direction:      DirectionOutbound,
		SourceKind:     j.SourceKind,
		SourceUserID:   j.SourceUserID,
		SourceRef:      j.SourceRef,
		Body:           j.Body,
		Attachments:    j.Attachments,
		ExternalID:     res.ExternalID,
		Status:         MsgSent,
		SentAt:         time.Now().UTC(),
	})
	if err != nil {
		w.failJob(ctx, j, "insert message: "+err.Error(), false)
		return
	}
	if msg == nil {
		// Already sent (deduped on external_id) — treat as success.
		_ = tx.Commit(ctx)
		_ = w.repo.MarkOutboundSent(ctx, j.ID, j.ConversationID) // benign — message_id won't be ours
		return
	}
	if err := tx.Commit(ctx); err != nil {
		w.failJob(ctx, j, "tx commit: "+err.Error(), false)
		return
	}

	if err := w.repo.MarkOutboundSent(ctx, j.ID, msg.ID); err != nil {
		w.log.Error("mark outbound sent", "job_id", j.ID, "err", err)
	}

	// 4. Notify SSE / future automations / future AI. StyleWorker's
	// ListenForNewReplies subscribes to this same event globally to learn
	// from staff-authored sends — kept there instead of here so it also
	// covers "typed directly on the linked phone" (WA-Web/TG-Web fromMe)
	// sends, which never go through this dispatch path at all.
	w.bus.Publish(ctx, Event{
		Kind:           EvtMessageSent,
		StudioID:       j.StudioID,
		ConversationID: conv.ID,
		MessageID:      &msg.ID,
	})
}

func (w *OutboundWorker) failJob(ctx context.Context, j OutboundJob, errMsg string, dead bool) {
	backoff := backoffFor(j.Attempts + 1)
	if dead {
		w.log.Error("outbound job dead-lettered", "job_id", j.ID, "attempts", j.Attempts+1, "err", errMsg)
	} else {
		w.log.Warn("outbound job failed; will retry",
			"job_id", j.ID, "attempts", j.Attempts+1, "backoff_s", backoff.Seconds(), "err", errMsg)
	}
	if err := w.repo.MarkOutboundFailed(ctx, j.ID, errMsg, backoff, dead); err != nil {
		w.log.Error("mark outbound failed", "job_id", j.ID, "err", err)
	}
}

// Exponential backoff capped at 30 minutes.
func backoffFor(attempts int) time.Duration {
	secs := math.Pow(2, float64(attempts))
	if secs > 1800 {
		secs = 1800
	}
	return time.Duration(secs) * time.Second
}

// testSender is a mock Sender for local development that logs messages instead of sending them.
type testSender struct{}

func (t *testSender) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []channels.Attachment) (*channels.SendResult, error) {
	// Always succeed in test mode with a fake external ID.
	return &channels.SendResult{
		ExternalID: "test-msg-" + time.Now().Format("20060102150405"),
	}, nil
}
