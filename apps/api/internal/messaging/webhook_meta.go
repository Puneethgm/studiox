package messaging

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
)

// MetaWebhookHandler exposes:
//
//   GET  /api/v1/webhooks/meta/whatsapp   — Meta verification handshake
//   POST /api/v1/webhooks/meta/whatsapp   — inbound events (messages, statuses)
//
// Both endpoints are single, app-level (one Meta App = one webhook URL = many
// connected studios). Studio-level routing happens via the phone_number_id in
// the payload, which we look up against channel_accounts.
type MetaWebhookHandler struct {
	svc         *Service
	verifyToken string // arbitrary string we set in Meta App config + here
	appSecret   string // Meta app secret — used to verify X-Hub-Signature-256
	log         *slog.Logger
}

func NewMetaWebhookHandler(svc *Service, verifyToken, appSecret string, log *slog.Logger) *MetaWebhookHandler {
	return &MetaWebhookHandler{
		svc:         svc,
		verifyToken: verifyToken,
		appSecret:   appSecret,
		log:         log,
	}
}

// GET handler: Meta sends ?hub.mode=subscribe&hub.verify_token=X&hub.challenge=Y
// We echo `hub.challenge` if `hub.verify_token` matches our configured token.
func (h *MetaWebhookHandler) Verify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("hub.mode")
	token := q.Get("hub.verify_token")
	challenge := q.Get("hub.challenge")

	if mode != "subscribe" || token != h.verifyToken {
		logger.FromCtx(r.Context(), h.log).Warn("meta webhook verify mismatch",
			"mode", mode, "token_len", len(token))
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(challenge))
}

// POST handler: receive WhatsApp events. Verify HMAC, parse, dispatch to service.
// We always 200 to Meta even on internal errors so they don't retry forever
// (errors are logged on our side).
func (h *MetaWebhookHandler) Receive(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("DEBUG: MetaWebhookHandler.Receive received POST request\n")
	log := logger.FromCtx(r.Context(), h.log).With("webhook", "meta_messaging")

	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20)) // 5 MB cap
	if err != nil {
		log.Error("read body", "err", err)
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

	fmt.Printf("DEBUG: MetaWebhookHandler.Receive raw body: %s\n", string(body))

	var payload channels.MetaWebhookPayload
	_ = json.Unmarshal(body, &payload) // unmarshal first to check for custom secret

	// Try to find the external ID of the channel from the webhook entries.
	var externalID string
	var kind ChannelKind = KindWhatsAppMeta

	for _, entry := range payload.Entry {
		if payload.Object == "whatsapp_business_account" {
			kind = KindWhatsAppMeta
			for _, change := range entry.Changes {
				if change.Field == "messages" {
					externalID = change.Value.Metadata.PhoneNumberID
					break
				}
			}
			if externalID == "" {
				externalID = entry.ID
			}
		} else if payload.Object == "instagram" || payload.Object == "page" {
			if payload.Object == "page" {
				kind = KindMessengerMeta
			} else {
				kind = KindInstagramMeta
			}
			for _, m := range entry.Messaging {
				if m.Recipient.ID != "" {
					externalID = m.Recipient.ID
					break
				}
				if m.Sender.ID != "" {
					externalID = m.Sender.ID
					break
				}
			}
		}
		if externalID != "" {
			break
		}
	}

	secretToUse := h.appSecret
	if externalID != "" {
		channel, err := h.svc.repo.GetChannelByExternalID(r.Context(), kind, externalID)
		if err == nil && channel != nil {
			customSecret, err := h.svc.repo.GetStudioMetaAppSecret(r.Context(), channel.StudioID)
			if err == nil && customSecret != "" {
				secretToUse = customSecret
			}
		}
	}

	if !h.verifySignature(r.Header.Get("X-Hub-Signature-256"), body, secretToUse) {
		log.Warn("invalid signature on meta webhook — rejecting")
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}

	log.Info("received meta webhook", "object", payload.Object, "entries", len(payload.Entry))

	for _, entry := range payload.Entry {
		// 1. Handle Instagram DMs and Facebook Messenger (messaging array)
		if payload.Object == "instagram" || payload.Object == "page" {
			kind := KindInstagramMeta
			if payload.Object == "page" {
				kind = KindMessengerMeta
			}
			for _, m := range entry.Messaging {
				if err := h.svc.HandleInboundMessaging(r.Context(), kind, m); err != nil {
					log.Error("handle inbound messaging", "err", err, "object", payload.Object)
				}
			}
			continue
		}

		// 2. Handle WhatsApp (changes array)
		if payload.Object == "whatsapp_business_account" {
			for _, change := range entry.Changes {
				if change.Field != "messages" {
					continue
				}
				value := change.Value

				contactsByWAID := map[string]*channels.WhatsAppWebhookContact{}
				for i := range value.Contacts {
					c := value.Contacts[i]
					contactsByWAID[c.WAID] = &c
				}

				for _, msg := range value.Messages {
					if err := h.svc.HandleInboundWhatsAppMessage(r.Context(),
						entry.ID, value.Metadata, contactsByWAID[msg.From], msg); err != nil {
						log.Error("handle inbound wa", "err", err, "from", msg.From, "id", msg.ID)
					}
				}

				for _, st := range value.Statuses {
					if err := h.svc.HandleStatus(r.Context(), st); err != nil {
						log.Error("handle status", "err", err, "id", st.ID)
					}
				}
			}
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// verifySignature: Meta signs the raw body with HMAC-SHA256 using the App
// Secret. Header format: "sha256=<hex>". Constant-time compare.
func (h *MetaWebhookHandler) verifySignature(header string, body []byte, secret string) bool {
	if os.Getenv("API_ENV") == "local" {
		fmt.Printf("DEBUG: MetaWebhookHandler.verifySignature bypassed in local dev mode (header=%s)\n", header)
		return true
	}
	if secret == "" {
		// Misconfiguration: refuse rather than silently accept.
		return false
	}
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(header[len("sha256="):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return subtle.ConstantTimeCompare(provided, expected) == 1
}
