package messaging

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"time"

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
	log       *slog.Logger
}

const (
	workerPollInterval = 2 * time.Second
	workerBatchSize    = 10
	maxAttempts        = 6
)

func NewOutboundWorker(repo *Repo, bus Bus, whatsapp, messenger channels.Sender, log *slog.Logger) *OutboundWorker {
	return &OutboundWorker{repo: repo, bus: bus, whatsapp: whatsapp, messenger: messenger, log: log}
}

func (w *OutboundWorker) Run(ctx context.Context) {
	w.log.Info("outbound worker started", "poll", workerPollInterval, "batch", workerBatchSize)
	t := time.NewTicker(workerPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("outbound worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
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

	var sender channels.Sender
	switch channel.Kind {
	case KindWhatsAppMeta:
		if isLocalDev && channel.AccessToken == "" {
			w.log.Info("test mode: using mock sender for WA", "job_id", j.ID)
			sender = &testSender{}
		} else {
			sender = w.whatsapp
		}
	case KindInstagramMeta, KindMessengerMeta:
		if isLocalDev && channel.AccessToken == "" {
			w.log.Info("test mode: using mock sender for Meta Messaging", "job_id", j.ID)
			sender = &testSender{}
		} else {
			sender = w.messenger
		}
	default:
		w.failJob(ctx, j, "no sender for channel kind: "+string(channel.Kind), true)
		return
	}

	// 2. Send via the channel adapter.
	res, err := sender.SendText(ctx, channel.AccessToken, channel.ExternalID, conv.ContactValue, j.Body)
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

	// 4. Notify SSE / future automations / future AI.
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

func (t *testSender) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string) (*channels.SendResult, error) {
	// Always succeed in test mode with a fake external ID.
	return &channels.SendResult{
		ExternalID: "test-msg-" + time.Now().Format("20060102150405"),
	}, nil
}
