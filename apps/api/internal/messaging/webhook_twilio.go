package messaging

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // Twilio's signature scheme is HMAC-SHA1; not our choice
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/projectx/api/internal/platform/httpx"
)

type TwilioWebhookHandler struct {
	svc           *Service
	publicBaseURL string // this API's externally reachable base URL, used to rebuild the URL Twilio signed
	log           *slog.Logger
}

func NewTwilioWebhookHandler(svc *Service, publicBaseURL string, log *slog.Logger) *TwilioWebhookHandler {
	return &TwilioWebhookHandler{
		svc:           svc,
		publicBaseURL: publicBaseURL,
		log:           log,
	}
}

// twilioSignatureValid implements Twilio's request validation: the signature is
// base64(HMAC-SHA1(authToken, url + for each POST param sorted by name: name+value)).
// It is valid if it matches for any candidate URL (Twilio signs the exact URL
// configured in its console, which behind a proxy differs from r.URL).
func twilioSignatureValid(authToken, signature string, candidateURLs []string, params url.Values) bool {
	if authToken == "" || signature == "" {
		return false
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	provided := []byte(signature)
	for _, u := range candidateURLs {
		var b strings.Builder
		b.WriteString(u)
		for _, k := range keys {
			for _, v := range params[k] {
				b.WriteString(k)
				b.WriteString(v)
			}
		}
		mac := hmac.New(sha1.New, []byte(authToken))
		mac.Write([]byte(b.String()))
		expected := []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		if subtle.ConstantTimeCompare(provided, expected) == 1 {
			return true
		}
	}
	return false
}

// twilioCandidateURLs rebuilds the URL Twilio would have signed: from the
// configured public base URL, and from the forwarded scheme/host headers.
func (h *TwilioWebhookHandler) twilioCandidateURLs(r *http.Request) []string {
	var out []string
	if base := strings.TrimRight(h.publicBaseURL, "/"); base != "" {
		out = append(out, base+r.URL.RequestURI())
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	out = append(out, scheme+"://"+host+r.URL.RequestURI())
	return out
}

// HandleInbound godoc
//
//	@Summary		Receive inbound Twilio SMS webhook
//	@Description	Receives inbound SMS (and any MMS media attachments) from Twilio as an application/x-www-form-urlencoded webhook, then dispatches the message to the messaging service. Responds with an empty TwiML document. Always responds 200 on downstream handling errors so Twilio doesn't retry indefinitely. No session/API-key auth — authenticated by Twilio's `X-Twilio-Signature` (HMAC-SHA1, using the matching channel's auth token); requests with a missing or invalid signature get 403.
//	@Tags			Webhooks
//	@Accept			x-www-form-urlencoded
//	@Produce		xml
//	@Success		200	{string}	string	"empty TwiML response"
//	@Failure		400	{object}	httpx.ErrorResponse	"invalid form data or missing required fields"
//	@Router			/api/v1/webhooks/twilio [post]
func (h *TwilioWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.log.Warn("failed to parse twilio webhook form", "err", err)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid form data")
		return
	}

	// Authenticate the request before trusting any field. The auth token belongs to
	// the studio's channel for the Twilio number the message was sent to. Stored as
	// "accountSID:authToken".
	toNumber := r.PostFormValue("To")
	channel, err := h.svc.repo.GetChannelByExternalID(r.Context(), KindSMS, toNumber)
	if err != nil {
		h.log.Warn("twilio webhook for unknown channel, rejecting", "to", toNumber)
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "invalid signature")
		return
	}
	authToken := channel.AccessToken
	if i := strings.Index(authToken, ":"); i >= 0 {
		authToken = authToken[i+1:]
	}
	if !twilioSignatureValid(authToken, r.Header.Get("X-Twilio-Signature"), h.twilioCandidateURLs(r), r.PostForm) {
		h.log.Warn("twilio webhook signature invalid, rejecting", "to", toNumber)
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "invalid signature")
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
