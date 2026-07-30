package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func withTelegramTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	original := TelegramAPIBaseURL
	TelegramAPIBaseURL = srv.URL
	t.Cleanup(func() { TelegramAPIBaseURL = original })
}

func TestTelegramSender_SendText_Success(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	withTelegramTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":123}}`))
	})

	creds, _ := json.Marshal(TelegramCredentials{BotToken: "123456:ABC-token", WebhookSecret: "shh"})
	s := NewTelegramSender()
	res, err := s.SendText(context.Background(), string(creds), "unused", "987654321", "hello there", nil)
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if res.ExternalID != "123" {
		t.Errorf("ExternalID = %q, want %q", res.ExternalID, "123")
	}
	if gotPath != "/bot123456:ABC-token/sendMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "987654321" {
		t.Errorf("chat_id = %v", gotBody["chat_id"])
	}
	if gotBody["text"] != "hello there" {
		t.Errorf("text = %v", gotBody["text"])
	}
}

func TestTelegramSender_SendText_InvalidToken(t *testing.T) {
	withTelegramTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
	})

	creds, _ := json.Marshal(TelegramCredentials{BotToken: "bad-token"})
	s := NewTelegramSender()
	_, err := s.SendText(context.Background(), string(creds), "unused", "987654321", "hi", nil)
	if err != ErrInvalidCredentials {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestTelegramSender_SendText_EmptyToken(t *testing.T) {
	s := NewTelegramSender()
	_, err := s.SendText(context.Background(), "", "unused", "987654321", "hi", nil)
	if err != ErrInvalidCredentials {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestTelegramGetMe_Success(t *testing.T) {
	withTelegramTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123456:ABC-token/getMe" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":42,"username":"my_studio_bot"}}`))
	})

	info, err := TelegramGetMe(context.Background(), nil, "123456:ABC-token")
	if err != nil {
		t.Fatalf("TelegramGetMe: %v", err)
	}
	if info.Username != "my_studio_bot" {
		t.Errorf("username = %q", info.Username)
	}
	if info.ID != 42 {
		t.Errorf("id = %d", info.ID)
	}
}

func TestTelegramSetWebhook_Success(t *testing.T) {
	var gotBody map[string]any
	withTelegramTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123456:ABC-token/setWebhook" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})

	err := TelegramSetWebhook(context.Background(), nil, "123456:ABC-token", "https://example.com/webhooks/telegram/123456:ABC-token", "shh")
	if err != nil {
		t.Fatalf("TelegramSetWebhook: %v", err)
	}
	if gotBody["url"] != "https://example.com/webhooks/telegram/123456:ABC-token" {
		t.Errorf("url = %v", gotBody["url"])
	}
	if gotBody["secret_token"] != "shh" {
		t.Errorf("secret_token = %v", gotBody["secret_token"])
	}
}

func TestTelegramSender_SendText_LocalAttachment_UploadsRealBytes(t *testing.T) {
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		t.Fatalf("mkdir uploads: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll("uploads") })
	localPath := filepath.Join("uploads", "test-photo.jpg")
	if err := os.WriteFile(localPath, []byte("fake-jpeg-bytes"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	var gotPath string
	var gotCaption string
	var gotFileBytes []byte
	withTelegramTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotCaption = r.FormValue("caption")
		file, _, err := r.FormFile("photo")
		if err != nil {
			t.Fatalf("read photo field: %v", err)
		}
		defer file.Close()
		gotFileBytes = make([]byte, 32)
		n, _ := file.Read(gotFileBytes)
		gotFileBytes = gotFileBytes[:n]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":99}}`))
	})

	creds, _ := json.Marshal(TelegramCredentials{BotToken: "123456:ABC-token"})
	s := NewTelegramSender()
	res, err := s.SendText(context.Background(), string(creds), "unused", "987654321", "here's a photo", []Attachment{
		{Type: "image", URL: "/uploads/test-photo.jpg"},
	})
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if res.ExternalID != "99" {
		t.Errorf("ExternalID = %q, want 99", res.ExternalID)
	}
	if gotPath != "/bot123456:ABC-token/sendPhoto" {
		t.Errorf("path = %q, want sendPhoto", gotPath)
	}
	if gotCaption != "here's a photo" {
		t.Errorf("caption = %q", gotCaption)
	}
	if string(gotFileBytes) != "fake-jpeg-bytes" {
		t.Errorf("uploaded bytes = %q, want the real local file contents (not a URL reference)", gotFileBytes)
	}
}

func TestTelegramSender_SendText_ExternalURLAttachment_PassesLinkNotBytes(t *testing.T) {
	var gotDocumentField string
	withTelegramTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotDocumentField = r.FormValue("document")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":100}}`))
	})

	creds, _ := json.Marshal(TelegramCredentials{BotToken: "123456:ABC-token"})
	s := NewTelegramSender()
	_, err := s.SendText(context.Background(), string(creds), "unused", "987654321", "", []Attachment{
		{Type: "document", URL: "https://cdn.example.com/brochure.pdf"},
	})
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if gotDocumentField != "https://cdn.example.com/brochure.pdf" {
		t.Errorf("document field = %q, want the external URL passed through directly", gotDocumentField)
	}
}
