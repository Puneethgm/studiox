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
	"strings"
	"time"
)

// MetaMessenger talks to Meta's Messenger/Instagram Graph API.
type MetaMessenger struct {
	apiVersion string
	httpClient *http.Client
}

func NewMetaMessenger(apiVersion string) *MetaMessenger {
	if apiVersion == "" {
		apiVersion = "v21.0"
	}
	return &MetaMessenger{
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// uploadMediaToMeta reads a local file from disk, uploads it to Meta's
// Messenger message_attachments API, and returns the attachment_id.
func (m *MetaMessenger) uploadMediaToMeta(ctx context.Context, accessToken, pageID, localPath, mediaType string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open local file %q: %w", localPath, err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(localPath))
	mimeType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".mp4":
		mimeType = "video/mp4"
	case ".mov":
		mimeType = "video/quicktime"
	case ".pdf":
		mimeType = "application/pdf"
	case ".doc":
		mimeType = "application/msword"
	case ".docx":
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	msgPayload := fmt.Sprintf(`{"attachment":{"type":"%s", "payload":{"is_reusable":true}}}`, mediaType)
	if fw, err := mw.CreateFormField("message"); err == nil {
		_, _ = fw.Write([]byte(msgPayload))
	}

	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="filedata"; filename="%s"`, filepath.Base(localPath)),
	}
	partHeader["Content-Type"] = []string{mimeType}

	part, err := mw.CreatePart(partHeader)
	if err != nil {
		return "", fmt.Errorf("create form part: %w", err)
	}
	if _, err = io.Copy(part, f); err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}
	mw.Close()

	uploadURL := fmt.Sprintf("%s/%s/%s/message_attachments", MetaGraphBaseURL, m.apiVersion, pageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("media upload http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		isLocalDev := os.Getenv("API_ENV") == "local"
		if isLocalDev {
			fmt.Printf("[Meta Messenger Media Upload Mapped to Mock] HTTP Status %d: %s\n", resp.StatusCode, string(respBody))
			return "mock_attachment_id_" + time.Now().Format("20060102150405"), nil
		}
		return "", fmt.Errorf("meta media upload: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var ok struct {
		AttachmentID string `json:"attachment_id"`
	}
	if err := json.Unmarshal(respBody, &ok); err != nil || ok.AttachmentID == "" {
		return "", fmt.Errorf("meta media upload: unexpected response: %s", string(respBody))
	}
	return ok.AttachmentID, nil
}

// SendText: POST /{page_id}/messages
// https://developers.facebook.com/docs/messenger-platform/reference/send-api
func (m *MetaMessenger) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []Attachment) (*SendResult, error) {
	if os.Getenv("API_ENV") == "local" && (accessToken == "" || accessToken == "test") {
		return &SendResult{
			ExternalID: "mid.test-" + time.Now().Format("20060102150405"),
		}, nil
	}
	if accessToken == "" {
		return nil, ErrInvalidCredentials
	}

	url := fmt.Sprintf("%s/%s/%s/messages", MetaGraphBaseURL, m.apiVersion, channelExternalID)

	var lastResult *SendResult
	if len(attachments) > 0 && attachments[0].URL != "" {
		mediaType := attachments[0].Type
		if mediaType == "" {
			mediaType = "image"
		}
		if mediaType == "document" {
			mediaType = "file"
		}

		attURL := attachments[0].URL
		var payload map[string]any

		if strings.HasPrefix(attURL, "/uploads/") {
			localPath := filepath.Join("uploads", strings.TrimPrefix(attURL, "/uploads/"))
			attID, uploadErr := m.uploadMediaToMeta(ctx, accessToken, channelExternalID, localPath, mediaType)
			if uploadErr != nil {
				fmt.Printf("[WARN] Meta Messenger media upload failed: %v. Falling back to URL.\n", uploadErr)
				payload = map[string]any{
					"recipient": map[string]string{"id": recipient},
					"message": map[string]any{
						"attachment": map[string]any{
							"type": mediaType,
							"payload": map[string]any{
								"url":         attURL,
								"is_reusable": true,
							},
						},
					},
				}
			} else {
				payload = map[string]any{
					"recipient": map[string]string{"id": recipient},
					"message": map[string]any{
						"attachment": map[string]any{
							"type": mediaType,
							"payload": map[string]any{
								"attachment_id": attID,
							},
						},
					},
				}
			}
		} else {
			payload = map[string]any{
				"recipient": map[string]string{"id": recipient},
				"message": map[string]any{
					"attachment": map[string]any{
						"type": mediaType,
						"payload": map[string]any{
							"url":         attURL,
							"is_reusable": true,
						},
					},
				},
			}
		}
		res, err := m.sendPayload(ctx, accessToken, url, payload)
		if err != nil {
			return nil, err
		}
		lastResult = res
	}

	if body != "" {
		useButtons := false
		var btnBody string
		var quickReplies []map[string]any

		lines := strings.Split(body, "\n")
		var parsedButtons []string
		var cleanBodyLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "1. "):
				parsedButtons = append(parsedButtons, strings.TrimSpace(trimmed[3:]))
			case strings.HasPrefix(trimmed, "2. "):
				parsedButtons = append(parsedButtons, strings.TrimSpace(trimmed[3:]))
			case strings.HasPrefix(trimmed, "3. "):
				parsedButtons = append(parsedButtons, strings.TrimSpace(trimmed[3:]))
			default:
				cleanBodyLines = append(cleanBodyLines, line)
			}
		}

		if len(parsedButtons) >= 1 && len(parsedButtons) <= 3 {
			useButtons = true
			btnBody = strings.TrimSpace(strings.Join(cleanBodyLines, "\n"))
			if btnBody == "" {
				btnBody = "Please choose an option:"
			}
			for idx, btnText := range parsedButtons {
				if len(btnText) > 20 {
					btnText = btnText[:20]
				}
				quickReplies = append(quickReplies, map[string]any{
					"content_type": "text",
					"title":        btnText,
					"payload":      fmt.Sprintf("choice_%d", idx+1),
				})
			}
		}

		var msgObj map[string]any
		if useButtons {
			msgObj = map[string]any{
				"text":          btnBody,
				"quick_replies": quickReplies,
			}
		} else {
			msgObj = map[string]any{
				"text": body,
			}
		}

		payload := map[string]any{
			"recipient":      map[string]string{"id": recipient},
			"messaging_type": "RESPONSE",
			"message":        msgObj,
		}
		res, err := m.sendPayload(ctx, accessToken, url, payload)
		if err != nil {
			return nil, err
		}
		lastResult = res
	}

	if lastResult == nil {
		return nil, fmt.Errorf("empty message body and attachments")
	}

	return lastResult, nil
}

func (m *MetaMessenger) sendPayload(ctx context.Context, accessToken, url string, payload map[string]any) (*SendResult, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var errEnv struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errEnv)
		if resp.StatusCode == 401 || errEnv.Error.Code == 190 {
			return nil, ErrInvalidCredentials
		}
		isLocalDev := os.Getenv("API_ENV") == "local"
		if isLocalDev {
			fmt.Printf("[Meta Messenger API Error Mapped to Mock] HTTP Status %d, Error Body: %s\n", resp.StatusCode, string(respBody))
			return &SendResult{
				ExternalID: "mid.mock-" + time.Now().Format("20060102150405"),
			}, nil
		}
		return nil, fmt.Errorf("meta messenger send: HTTP %d: %s", resp.StatusCode, errEnv.Error.Message)
	}

	var ok struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(respBody, &ok); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &SendResult{ExternalID: ok.MessageID}, nil
}
