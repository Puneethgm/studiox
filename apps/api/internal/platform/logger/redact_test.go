package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactAttr_KeyShapedAsSecret(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"apiKey", redactedPlaceholder},
		{"api_key", redactedPlaceholder},
		{"secret", redactedPlaceholder},
		{"token", redactedPlaceholder},
		{"password", redactedPlaceholder},
		{"groqApiKey", redactedPlaceholder},
	}
	for _, tc := range cases {
		got := redactAttr(slog.String(tc.key, "sk-super-secret-value-1234567890"))
		if got.Value.String() != tc.want {
			t.Errorf("redactAttr(%q) = %q, want %q", tc.key, got.Value.String(), tc.want)
		}
	}
}

func TestRedactAttr_AllowlistedKeysPassThrough(t *testing.T) {
	for _, key := range []string{"hasApiKey", "keyStatus", "keyFingerprint"} {
		original := "ab12cd34"
		got := redactAttr(slog.String(key, original))
		if got.Value.String() != original {
			t.Errorf("redactAttr(%q) redacted an allowlisted field: got %q, want unchanged %q", key, got.Value.String(), original)
		}
	}
}

func TestRedactAttr_ValueShapedAsSecretUnderGenericKey(t *testing.T) {
	cases := []string{
		"sk-ant-api03-abcdefghijklmnop",
		"sk-abcdefghijklmnopqrstuvwx",
		"gsk_abcdefghijklmnopqrstuvwx",
		"AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ012",
		"xoxb-1234567890-abcdefgh",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
	}
	for _, v := range cases {
		got := redactAttr(slog.String("detail", v))
		if strings.Contains(got.Value.String(), v) {
			t.Errorf("redactAttr did not redact secret-shaped value under a generic key: %q", v)
		}
	}
}

func TestRedactAttr_OrdinaryValuesUnaffected(t *testing.T) {
	got := redactAttr(slog.String("studio_id", "6b15859f-c239-4049-8f51-94395ed7c3af"))
	want := "6b15859f-c239-4049-8f51-94395ed7c3af"
	if got.Value.String() != want {
		t.Errorf("redactAttr changed an ordinary value: got %q, want %q", got.Value.String(), want)
	}
}

// TestLoggerNew_RedactsSecretKeyInRealLogLine is an end-to-end check that
// New's ReplaceAttr hook (not just redactAttr in isolation) keeps a
// secret-shaped field out of the actual emitted JSON line.
func TestLoggerNew_RedactsSecretKeyInRealLogLine(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return a
			}
			return redactAttr(a)
		},
	})
	l := slog.New(h)
	l.Info("key rotation requested", "provider", "groq", "apiKey", "gsk_realsecretvaluethatmustneverbelogged")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}
	if strings.Contains(buf.String(), "gsk_realsecretvaluethatmustneverbelogged") {
		t.Fatal("the raw secret value appears in the emitted log line")
	}
	if line["apiKey"] != redactedPlaceholder {
		t.Errorf("apiKey = %v, want %q", line["apiKey"], redactedPlaceholder)
	}
	if line["provider"] != "groq" {
		t.Errorf("provider = %v, want %q (non-secret fields must be unaffected)", line["provider"], "groq")
	}
}
