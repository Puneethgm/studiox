package messaging

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/projectx/api/internal/platform/httpx"
)

type TwilioWebhookHandler struct {
	svc *Service
	log *slog.Logger
}

func NewTwilioWebhookHandler(svc *Service, log *slog.Logger) *TwilioWebhookHandler {
	return &TwilioWebhookHandler{
		svc: svc,
		log: log,
	}
}

func (h *TwilioWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.log.Warn("failed to parse twilio webhook form", "err", err)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid form data")
		return
	}

	from := r.FormValue("From")
	to := r.FormValue("To")
	body := r.FormValue("Body")
	messageSid := r.FormValue("MessageSid")

	if from == "" || to == "" || messageSid == "" {
		h.log.Warn("twilio webhook missing required fields")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "missing fields")
		return
	}

	// Remove '+' from phone numbers to match our internal format if desired,
	// or keep it if we standardize on E.164.
	// We'll leave them as they come from Twilio (e.g. +1234567890).

	// Parse media attachments
	var attachments []Attachment
	numMediaStr := r.FormValue("NumMedia")
	if numMediaStr != "" && numMediaStr != "0" {
		if count, err := strconv.Atoi(numMediaStr); err == nil {
			for i := 0; i < count; i++ {
				url := r.FormValue("MediaUrl" + strconv.Itoa(i))
				mimeType := r.FormValue("MediaContentType" + strconv.Itoa(i))
				if url != "" {
					attType := "image"
					if len(mimeType) > 5 && mimeType[:5] == "video" {
						attType = "video"
					} else if len(mimeType) > 5 && mimeType[:5] == "audio" {
						attType = "audio"
					} else if len(mimeType) > 11 && mimeType[:11] == "application" {
						attType = "document"
					}
					attachments = append(attachments, Attachment{
						Type: attType,
						URL:  url,
						Mime: mimeType,
						Name: "attachment",
					})
				}
			}
		}
	}

	h.log.Info("received twilio inbound sms", "from", from, "to", to, "body_len", len(body), "media_count", len(attachments))

	if err := h.svc.HandleInboundSMS(r.Context(), messageSid, from, to, body, attachments); err != nil {
		h.log.Error("failed to handle inbound twilio sms", "err", err, "msg_sid", messageSid)
		// Return 200 anyway so Twilio doesn't retry indefinitely for logic errors
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	// Return empty TwiML response
	_, _ = w.Write([]byte("<Response></Response>"))
}
