package messaging

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
		ForUserId       string `json:"for_user_id"`
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

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
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
