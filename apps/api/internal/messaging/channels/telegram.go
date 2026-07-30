package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TelegramAPIBaseURL is the Telegram Bot API base — var so tests can point
// it at an httptest server.
var TelegramAPIBaseURL = "https://api.telegram.org"

// TelegramSender talks to the Telegram Bot API. accessToken is the raw bot
// token issued by @BotFather (no OAuth, no BSP).
type TelegramSender struct {
	httpClient *http.Client
}

func NewTelegramSender() *TelegramSender {
	return &TelegramSender{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type telegramAPIResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Result      struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
}

// TelegramCredentials is what's stored (as JSON) in channel_accounts'
// encrypted access_token column. Bundling the webhook secret alongside the
// bot token — rather than a separate plaintext column — mirrors how x.go
// packs its four OAuth1 secrets into the one encrypted field.
type TelegramCredentials struct {
	BotToken      string `json:"bot_token"`
	WebhookSecret string `json:"webhook_secret"`
}

// SendText: POST /bot<token>/sendMessage, or one of sendPhoto/sendVideo/
// sendAudio/sendDocument when there are attachments (Telegram has no
// single "send with attachments" call — each media type is its own
// endpoint). channelExternalID is unused — Telegram routes by bot token
// alone, unlike Meta's per-page IDs.
func (s *TelegramSender) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []Attachment) (*SendResult, error) {
	if accessToken == "" {
		return nil, ErrInvalidCredentials
	}
	var creds TelegramCredentials
	if err := json.Unmarshal([]byte(accessToken), &creds); err != nil || creds.BotToken == "" {
		return nil, fmt.Errorf("invalid telegram channel credentials: %w", err)
	}

	if len(attachments) > 0 {
		return s.sendAttachments(ctx, creds.BotToken, recipient, body, attachments)
	}

	payload := map[string]any{
		"chat_id": recipient,
		"text":    body,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", TelegramAPIBaseURL, creds.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var parsed telegramAPIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("telegram decode: %w", err)
	}

	if !parsed.OK {
		if resp.StatusCode == http.StatusUnauthorized || parsed.ErrorCode == http.StatusUnauthorized {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("telegram send: %s", parsed.Description)
	}

	return &SendResult{ExternalID: strconv.FormatInt(parsed.Result.MessageID, 10)}, nil
}

// telegramMediaMethod maps our generic Attachment.Type to the Bot API
// endpoint + form field name Telegram expects the file under.
func telegramMediaMethod(attType string) (method, field string) {
	switch strings.ToLower(attType) {
	case "image":
		return "sendPhoto", "photo"
	case "video":
		return "sendVideo", "video"
	case "audio":
		return "sendAudio", "audio"
	default:
		return "sendDocument", "document"
	}
}

// sendAttachments sends each attachment via its dedicated Bot API method —
// caption (the message body) goes on the first one, matching the
// one-caption-per-batch convention the other channel adapters already use.
// Local /uploads/ files are uploaded as real multipart bytes (mirrors
// meta_messenger.go's uploadMediaToMeta) rather than passed as a URL —
// Telegram's servers can't reach a bare "localhost"/internal-Docker-hostname
// URL, so relying on a link only works for genuinely external attachment
// URLs.
func (s *TelegramSender) sendAttachments(ctx context.Context, botToken, recipient, body string, attachments []Attachment) (*SendResult, error) {
	var lastResult *SendResult
	for i, att := range attachments {
		caption := ""
		if i == 0 {
			caption = body
		}
		res, err := s.sendOneAttachment(ctx, botToken, recipient, caption, att)
		if err != nil {
			return nil, err
		}
		lastResult = res
	}
	return lastResult, nil
}

func (s *TelegramSender) sendOneAttachment(ctx context.Context, botToken, recipient, caption string, att Attachment) (*SendResult, error) {
	method, field := telegramMediaMethod(att.Type)
	url := fmt.Sprintf("%s/bot%s/%s", TelegramAPIBaseURL, botToken, method)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", recipient)
	if caption != "" {
		_ = mw.WriteField("caption", caption)
	}

	if strings.HasPrefix(att.URL, "/uploads/") {
		localPath := filepath.Join("uploads", strings.TrimPrefix(att.URL, "/uploads/"))
		f, err := os.Open(localPath)
		if err != nil {
			return nil, fmt.Errorf("open local attachment %q: %w", localPath, err)
		}
		defer f.Close()
		fw, err := mw.CreateFormFile(field, filepath.Base(localPath))
		if err != nil {
			return nil, fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(fw, f); err != nil {
			return nil, fmt.Errorf("copy attachment: %w", err)
		}
	} else {
		// A genuinely external URL — Telegram fetches it server-side, no
		// upload needed on our end.
		_ = mw.WriteField(field, att.URL)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram media http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var parsed telegramAPIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("telegram media decode: %w", err)
	}
	if !parsed.OK {
		if resp.StatusCode == http.StatusUnauthorized || parsed.ErrorCode == http.StatusUnauthorized {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("telegram send %s: %s", method, parsed.Description)
	}
	return &SendResult{ExternalID: strconv.FormatInt(parsed.Result.MessageID, 10)}, nil
}

// TelegramSetWebhook registers the studio's per-channel webhook URL with
// Telegram, called once at connect-time (not part of the Sender interface —
// it's a connection-setup step, not an outbound send).
func TelegramSetWebhook(ctx context.Context, httpClient *http.Client, botToken, webhookURL, secretToken string) error {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	payload := map[string]any{
		"url":          webhookURL,
		"secret_token": secretToken,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/setWebhook", TelegramAPIBaseURL, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram setWebhook http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// setWebhook's "result" is a bare bool, unlike sendMessage's — separate
	// shape from telegramAPIResponse.
	var parsed struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Result      bool   `json:"result"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("telegram setWebhook decode: %w", err)
	}
	if !parsed.OK {
		if resp.StatusCode == http.StatusUnauthorized || parsed.ErrorCode == http.StatusUnauthorized {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("telegram setWebhook: %s", parsed.Description)
	}
	return nil
}

// TelegramBotInfo is the subset of getMe's result we need at connect time.
type TelegramBotInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// TelegramGetMe validates a bot token and returns the bot's identity, used
// at connect-time to confirm the token is real before storing it. ID is
// public (not secret) and becomes channel_accounts.external_id, so inbound
// webhooks can be routed by ID alone without ever putting the bot token in
// a URL or log line.
func TelegramGetMe(ctx context.Context, httpClient *http.Client, botToken string) (TelegramBotInfo, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	url := fmt.Sprintf("%s/bot%s/getMe", TelegramAPIBaseURL, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return TelegramBotInfo{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return TelegramBotInfo{}, fmt.Errorf("telegram getMe http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var parsed struct {
		OK          bool            `json:"ok"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Result      TelegramBotInfo `json:"result"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return TelegramBotInfo{}, fmt.Errorf("telegram getMe decode: %w", err)
	}
	if !parsed.OK {
		if resp.StatusCode == http.StatusUnauthorized || parsed.ErrorCode == http.StatusUnauthorized {
			return TelegramBotInfo{}, ErrInvalidCredentials
		}
		return TelegramBotInfo{}, fmt.Errorf("telegram getMe: %s", parsed.Description)
	}
	return parsed.Result, nil
}

// TelegramDownloadFile pulls an inbound attachment down to local disk,
// mirroring downloadWhatsAppMedia's two-step shape (Telegram's Bot API
// works the same way: resolve a file_id to a temporary file_path via
// getFile, then download from a second URL built from that path — the
// file_id alone isn't directly fetchable). Saved under ./uploads, same
// convention every other channel's inbound media uses, so the existing
// /uploads/* static file server and message.attachments.url handling need
// no Telegram-specific changes downstream.
func TelegramDownloadFile(ctx context.Context, httpClient *http.Client, botToken, fileID, fileName, messageID string) (localURL, savedName string, err error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	getFileURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", TelegramAPIBaseURL, botToken, fileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getFileURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("telegram getFile http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var parsed struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Result      struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("telegram getFile decode: %w", err)
	}
	if !parsed.OK {
		if resp.StatusCode == http.StatusUnauthorized || parsed.ErrorCode == http.StatusUnauthorized {
			return "", "", ErrInvalidCredentials
		}
		return "", "", fmt.Errorf("telegram getFile: %s", parsed.Description)
	}
	if parsed.Result.FilePath == "" {
		return "", "", fmt.Errorf("telegram getFile: empty file_path")
	}

	// Note: this URL is bot-token-bearing, unlike every other Telegram URL
	// in this codebase — Bot API file downloads have no separate auth
	// mechanism. Fetched server-side only, never exposed to a client.
	downloadURL := fmt.Sprintf("%s/file/bot%s/%s", TelegramAPIBaseURL, botToken, parsed.Result.FilePath)
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", "", err
	}
	dlResp, err := httpClient.Do(dlReq)
	if err != nil {
		return "", "", fmt.Errorf("telegram file download: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(dlResp.Body)
		return "", "", fmt.Errorf("telegram file download HTTP %d: %s", dlResp.StatusCode, errBody)
	}

	if err := os.MkdirAll("uploads", 0o755); err != nil {
		return "", "", fmt.Errorf("create uploads dir: %w", err)
	}

	ext := filepath.Ext(parsed.Result.FilePath)
	if ext == "" {
		ext = filepath.Ext(fileName)
	}
	if ext == "" {
		ext = ".bin"
	}
	savedName = fmt.Sprintf("telegram-%s%s", messageID, ext)
	outPath := filepath.Join("uploads", savedName)
	outFile, err := os.Create(outPath)
	if err != nil {
		return "", "", fmt.Errorf("create media file: %w", err)
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, dlResp.Body); err != nil {
		return "", "", fmt.Errorf("write media file: %w", err)
	}

	return "/uploads/" + savedName, savedName, nil
}

// TelegramUpdate is Telegram's inbound webhook payload (the subset we use).
// https://core.telegram.org/bots/api#update
type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message,omitempty"`
}

type TelegramMessage struct {
	MessageID int64         `json:"message_id"`
	Date      int64         `json:"date"`
	Text      string        `json:"text,omitempty"`
	Caption   string        `json:"caption,omitempty"`
	Chat      TelegramChat  `json:"chat"`
	From      *TelegramUser `json:"from,omitempty"`

	// Media — Telegram uses a distinct field per type rather than one
	// generic "attachment" shape. At most one of these is populated per
	// message (Telegram doesn't allow mixing, e.g. a photo+document in one
	// message — an album is multiple separate Update payloads instead).
	Photo    []TelegramPhotoSize `json:"photo,omitempty"` // multiple resolutions; last is largest
	Video    *TelegramFileBase   `json:"video,omitempty"`
	Audio    *TelegramFileBase   `json:"audio,omitempty"`
	Voice    *TelegramFileBase   `json:"voice,omitempty"`
	Document *TelegramFileBase   `json:"document,omitempty"`
}

// TelegramPhotoSize is one entry in Message.photo — Telegram sends the same
// image at several resolutions; TelegramMessage.LargestPhoto() picks the best.
type TelegramPhotoSize struct {
	FileID string `json:"file_id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// TelegramFileBase covers the fields shared by video/audio/voice/document —
// they differ only in a few type-specific extras we don't need here.
type TelegramFileBase struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type,omitempty"`
	FileName string `json:"file_name,omitempty"` // documents only
	FileSize int64  `json:"file_size,omitempty"`
}

// LargestPhoto returns the highest-resolution entry from Message.photo, or
// nil if this message has no photo. Telegram lists them smallest-first.
func (m *TelegramMessage) LargestPhoto() *TelegramPhotoSize {
	if len(m.Photo) == 0 {
		return nil
	}
	return &m.Photo[len(m.Photo)-1]
}

// Media returns (fileID, attachmentType, mimeHint, fileName) for whichever
// single media field is populated on this message, or ("", "", "", "") if
// it's a plain text message. attachmentType matches the Attachment.Type
// convention ("image"/"video"/"audio"/"document") used across every channel.
func (m *TelegramMessage) Media() (fileID, attachmentType, mimeHint, fileName string) {
	if p := m.LargestPhoto(); p != nil {
		return p.FileID, "image", "image/jpeg", ""
	}
	if m.Video != nil {
		return m.Video.FileID, "video", m.Video.MimeType, ""
	}
	if m.Voice != nil {
		return m.Voice.FileID, "audio", m.Voice.MimeType, ""
	}
	if m.Audio != nil {
		return m.Audio.FileID, "audio", m.Audio.MimeType, ""
	}
	if m.Document != nil {
		return m.Document.FileID, "document", m.Document.MimeType, m.Document.FileName
	}
	return "", "", "", ""
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}
