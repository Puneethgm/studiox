package messaging

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
)

// MetaWebhookHandler exposes:
//
//	GET  /api/v1/webhooks/meta/whatsapp   — Meta verification handshake
//	POST /api/v1/webhooks/meta/whatsapp   — inbound events (messages, statuses)
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

// Verify godoc
//
//	@Summary		Meta webhook verification handshake
//	@Description	Handles Meta's webhook subscription handshake: echoes back `hub.challenge` if `hub.mode=subscribe` and `hub.verify_token` matches our configured token, otherwise responds 403. Authenticated via the verify_token query param rather than a signature — no session/API-key auth.
//	@Tags			Webhooks
//	@Produce		plain
//	@Param			hub.mode			query		string	true	"Must be 'subscribe'"
//	@Param			hub.verify_token	query		string	true	"Must match the configured verify token"
//	@Param			hub.challenge		query		string	true	"Echoed back on success"
//	@Success		200					{string}	string	"hub.challenge value"
//	@Failure		403					{string}	string	"mode or token mismatch"
//	@Router			/api/v1/webhooks/meta/whatsapp [get]
//	@Router			/api/v1/webhooks/meta/messenger [get]
//	@Router			/api/v1/webhooks/meta/instagram [get]
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

// HandleDataDeletion godoc
//
//	@Summary		Meta data deletion callback
//	@Description	Handles Meta's GDPR/CCPA data-deletion request callback: decodes the `signed_request` payload and acknowledges it with a confirmation URL. Signature verification of the signed request is not yet implemented. No session/API-key auth — this is an unauthenticated callback URL configured in the Meta App dashboard.
//	@Tags			Webhooks
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/api/v1/webhooks/meta/data-deletion [post]
func (h *MetaWebhookHandler) HandleDataDeletion(w http.ResponseWriter, r *http.Request) {
	log := logger.FromCtx(r.Context(), h.log).With("webhook", "meta_data_deletion")

	var req struct {
		SignedRequest string `json:"signed_request"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("decode request", "err", err)
		httpx.JSON(w, http.StatusOK, map[string]string{"url": ""})
		return
	}

	if req.SignedRequest == "" {
		log.Warn("no signed request provided")
		httpx.JSON(w, http.StatusOK, map[string]string{"url": ""})
		return
	}

	// Parse the signed request (format: base64_signature.base64_payload)
	parts := strings.Split(req.SignedRequest, ".")
	if len(parts) != 2 {
		log.Warn("invalid signed request format")
		httpx.JSON(w, http.StatusOK, map[string]string{"url": ""})
		return
	}

	// For now, we just acknowledge the deletion request
	// In production, you would verify the signature using your app secret
	// and then delete the user's data from your database

	log.Info("data deletion request received", "request", req.SignedRequest)

	// Return confirmation to Meta
	httpx.JSON(w, http.StatusOK, map[string]string{
		"url": "https://1herosocial.ai/privacy",
	})
}

// Receive godoc
//
//	@Summary		Receive inbound Meta webhook events
//	@Description	Receives inbound WhatsApp/Messenger/Instagram events from Meta. Verifies the `X-Hub-Signature-256` HMAC (using the studio's custom app secret if one is configured, falling back to the app-level secret) before dispatching messages/statuses to the messaging service. Always responds 200 (even on internal errors) so Meta doesn't retry indefinitely; errors are logged server-side. No session/API-key auth — authenticated via the HMAC signature header instead.
//	@Tags			Webhooks
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{string}	string	"bad request body"
//	@Failure		401	{string}	string	"invalid signature"
//	@Router			/api/v1/webhooks/meta/whatsapp [post]
//	@Router			/api/v1/webhooks/meta/messenger [post]
//	@Router			/api/v1/webhooks/meta/instagram [post]
func (h *MetaWebhookHandler) Receive(w http.ResponseWriter, r *http.Request) {
	log := logger.FromCtx(r.Context(), h.log).With("webhook", "meta_messaging")

	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20)) // 5 MB cap
	if err != nil {
		log.Error("read body", "err", err)
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

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
