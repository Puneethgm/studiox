package messaging

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/projectx/api/internal/platform/httpx"
)

type TwitterWebhookHandler struct {
	svc *Service
	log *slog.Logger
}

func NewTwitterWebhookHandler(svc *Service, log *slog.Logger) *TwitterWebhookHandler {
	return &TwitterWebhookHandler{svc: svc, log: log}
}

// xSignatureValid checks X's `x-twitter-webhooks-signature` header:
// "sha256=" + base64(HMAC-SHA256(consumerSecret, rawBody)). It is valid if it
// matches any of the candidate secrets (one per connected X channel, since the
// payload doesn't say which app it belongs to before it is verified).
func xSignatureValid(header string, body []byte, secrets []string) bool {
	const prefix = "sha256="
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	provided := []byte(header)
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := []byte(prefix + base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		if subtle.ConstantTimeCompare(provided, expected) == 1 {
			return true
		}
	}
	return false
}

// activeXConsumerSecrets returns the consumer secret of every active X channel.
func (h *TwitterWebhookHandler) activeXConsumerSecrets(ctx context.Context) []string {
	rows, err := h.svc.repo.Pool().Query(ctx, "SELECT access_token_enc FROM channel_accounts WHERE kind = 'x_dm' AND status = 'active'")
	if err != nil {
		h.log.Error("load x channels for signature check", "err", err)
		return nil
	}
	defer rows.Close()
	var secrets []string
	for rows.Next() {
		var enc string
		if err := rows.Scan(&enc); err != nil {
			continue
		}
		dec, err := h.svc.repo.cipher.Decrypt(enc)
		if err != nil {
			continue
		}
		var keys struct {
			ConsumerSecret string `json:"consumer_secret"`
		}
		if json.Unmarshal([]byte(dec), &keys) == nil && keys.ConsumerSecret != "" {
			secrets = append(secrets, keys.ConsumerSecret)
		}
	}
	return secrets
}

// HandleInbound godoc
//
//	@Summary		X (Twitter) webhook CRC challenge and inbound DM events
//	@Description	Handles both halves of X's Account Activity webhook. On GET, responds to X's CRC challenge by HMAC-SHA256-signing the `crc_token` query param with the connected channel's consumer secret and returning the base64-encoded `response_token`. On POST, decodes the `direct_message_events` payload and dispatches each inbound (non-self) message_create event to the messaging service, always returning 200. No session/API-key auth — GET is authenticated implicitly by the shared consumer secret used to answer the CRC challenge; POST is verified against the `x-twitter-webhooks-signature` HMAC (consumer secret of any connected X channel); an invalid or missing signature gets 401.
//	@Tags			Webhooks
//	@Accept			json
//	@Produce		json
//	@Param			crc_token	query		string	false	"X CRC challenge token (GET only)"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"missing crc_token or no active X channel found (GET only)"
//	@Router			/api/v1/webhooks/x [get]
//	@Router			/api/v1/webhooks/x [post]
func (h *TwitterWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		crcToken := r.URL.Query().Get("crc_token")
		if crcToken == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid", "missing crc_token")
			return
		}

		channels, err := h.svc.repo.Pool().Query(r.Context(), "SELECT access_token_enc FROM channel_accounts WHERE kind = 'x_dm' AND status = 'active' LIMIT 1")
		if err != nil || !channels.Next() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid", "no active x channel found")
			return
		}
		var encToken string
		channels.Scan(&encToken)
		channels.Close()

		decToken, _ := h.svc.repo.cipher.Decrypt(encToken)
		var keys struct {
			ConsumerSecret string `json:"consumer_secret"`
		}
		json.Unmarshal([]byte(decToken), &keys)

		mac := hmac.New(sha256.New, []byte(keys.ConsumerSecret))
		mac.Write([]byte(crcToken))
		hash := "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))

		httpx.JSON(w, http.StatusOK, map[string]string{
			"response_token": hash,
		})
		return
	}

	var payload struct {
		ForUserId           string `json:"for_user_id"`
		DirectMessageEvents []struct {
			Type          string `json:"type"`
			MessageCreate struct {
				SenderId string `json:"sender_id"`
				Target   struct {
					RecipientId string `json:"recipient_id"`
				} `json:"target"`
				MessageData struct {
					Text string `json:"text"`
				} `json:"message_data"`
			} `json:"message_create"`
		} `json:"direct_message_events"`
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", "unreadable body")
		return
	}
	if !xSignatureValid(r.Header.Get("x-twitter-webhooks-signature"), rawBody, h.activeXConsumerSecrets(r.Context())) {
		h.log.Warn("x webhook signature invalid, rejecting")
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid signature")
		return
	}

	if err := json.Unmarshal(rawBody, &payload); err != nil {
		h.log.Error("failed to decode x webhook", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, event := range payload.DirectMessageEvents {
		if event.Type != "message_create" {
			continue
		}

		senderID := event.MessageCreate.SenderId
		recipientID := event.MessageCreate.Target.RecipientId
		text := event.MessageCreate.MessageData.Text

		if senderID == payload.ForUserId {
			continue
		}

		err := h.svc.HandleInboundSMS(r.Context(), "X_DM_DUMMY_SID", senderID, recipientID, text, nil)
		if err != nil {
			h.log.Error("failed to handle inbound x dm", "err", err, "sender", senderID)
		} else {
			h.log.Info("received x inbound dm", "from", senderID, "to", recipientID)
		}
	}

	w.WriteHeader(http.StatusOK)
}
