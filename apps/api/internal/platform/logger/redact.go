package logger

import (
	"log/slog"
	"regexp"
)

// secretShapedAttrKey flags a log attribute key that looks like it might
// carry a raw secret value — mirrors the response-field check in
// internal/studios/secret_response_scan_test.go, applied here to log
// attributes instead of HTTP response fields. This is a safety net: no
// call site should ever log a secret value, but one slipping through
// (a copy-pasted debug log, a future handler) should be redacted rather
// than reach stdout/Loki in the clear.
// platform-security-hardening task 0.7.
var secretShapedAttrKey = regexp.MustCompile(`(?i)(api)?key$|secret$|token$|password$|credential$`)

// redactAttrAllowlist lists attribute keys that match
// secretShapedAttrKey's pattern but are known-safe — a presence flag, a
// status, or a fingerprint that is itself the safe display value a raw
// secret was replaced with.
var redactAttrAllowlist = map[string]bool{
	"has_api_key":     true,
	"hasApiKey":       true,
	"key_status":      true,
	"keyStatus":       true,
	"key_fingerprint": true,
	"keyFingerprint":  true,
}

// secretShapedValuePattern is a value-based safety net for a secret
// logged under a key name the allowlist/denylist above doesn't catch
// (e.g. a generic "value" or "detail" field) — recognizable provider
// key/token prefixes and shapes seen in this codebase (Groq, Gemini,
// Claude, OpenAI, Slack-style bot tokens, JWT bearer tokens). Not
// exhaustive by design — the key-based check above is the primary
// defense; this catches the common, recognizable cases.
var secretShapedValuePattern = regexp.MustCompile(`(?:sk-ant-[A-Za-z0-9_-]{8,}|sk-[A-Za-z0-9_-]{16,}|gsk_[A-Za-z0-9_-]{16,}|AIzaSy[A-Za-z0-9_-]{16,}|xox[baprs]-[A-Za-z0-9-]{8,}|eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,})`)

const redactedPlaceholder = "[REDACTED]"

// redactAttr is the slog.HandlerOptions.ReplaceAttr hook applied to every
// log line this app writes (see New below). It never removes a field —
// only replaces a secret-shaped value with a fixed placeholder, so a log
// line stays structurally intact for querying while never carrying the
// secret itself.
func redactAttr(a slog.Attr) slog.Attr {
	if redactAttrAllowlist[a.Key] {
		return a
	}
	if secretShapedAttrKey.MatchString(a.Key) && a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, redactedPlaceholder)
	}
	if a.Value.Kind() == slog.KindString {
		if v := a.Value.String(); secretShapedValuePattern.MatchString(v) {
			return slog.String(a.Key, secretShapedValuePattern.ReplaceAllString(v, redactedPlaceholder))
		}
	}
	return a
}
