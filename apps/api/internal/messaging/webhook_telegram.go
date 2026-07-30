package messaging

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
)

// TelegramWebhookHandler exposes:
//
//	POST /api/v1/webhooks/telegram/{botID}
//
// Unlike Meta (one shared app-level webhook), each Telegram bot has its own
// webhook URL. {botID} is the bot's public numeric Telegram ID (returned by
// getMe at connect time) — never the bot token, so nothing secret ends up in
// the URL or access logs. Authenticity is instead verified via Telegram's
// X-Telegram-Bot-Api-Secret-Token header against a per-channel secret we
// generated and registered via setWebhook.
type TelegramWebhookHandler struct {
	svc *Service
	log *slog.Logger
}

func NewTelegramWebhookHandler(svc *Service, log *slog.Logger) *TelegramWebhookHandler {
	return &TelegramWebhookHandler{svc: svc, log: log}
}

func (h *TelegramWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	log := logger.FromCtx(r.Context(), h.log).With("webhook", "telegram")

	botID := chi.URLParam(r, "botID")
	if botID == "" {
		http.Error(w, "missing bot id", http.StatusBadRequest)
		return
	}

	channel, err := h.svc.repo.GetChannelByExternalID(r.Context(), KindTelegram, botID)
	if err != nil || channel == nil {
		log.Warn("telegram webhook for unknown bot", "bot_id", botID)
		// 200 anyway — an unknown/disconnected channel isn't a payload error;
		// no point making Telegram retry a bot we've already dropped.
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	var creds channels.TelegramCredentials
	if err := json.Unmarshal([]byte(channel.AccessToken), &creds); err != nil {
		log.Error("decode telegram credentials", "err", err, "channel_id", channel.ID)
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	provided := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(creds.WebhookSecret)) != 1 {
		log.Warn("invalid telegram webhook secret token", "channel_id", channel.ID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB cap
	if err != nil {
		log.Error("read body", "err", err)
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

	var upd channels.TelegramUpdate
	if err := json.Unmarshal(body, &upd); err != nil {
		log.Error("decode telegram update", "err", err)
		// Malformed body isn't worth a retry either.
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if err := h.svc.HandleInboundTelegramMessage(r.Context(), botID, upd); err != nil {
		log.Error("handle inbound telegram message", "err", err, "update_id", upd.UpdateID)
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
