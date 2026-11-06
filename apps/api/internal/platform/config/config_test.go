package config

import (
	"strings"
	"testing"
)

// setMinimalRequiredEnv sets the env vars Load() hard-requires
// (JWT_SECRET, TOKEN_ENCRYPTION_KEY) so these tests exercise only the
// COOKIE_SECURE/API_ENV interaction, not unrelated required-field errors.
func setMinimalRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "dGhpc2lzYTMyYnl0ZWJhc2U2NGVuY29kZWRrZXkh")
	t.Setenv("WA_WEB_INTERNAL_KEY", "wa-web-test-key")
	t.Setenv("TG_WEB_INTERNAL_KEY", "tg-web-test-key")
}

func TestLoad_RefusesInsecureCookieOutsideLocal(t *testing.T) {
	setMinimalRequiredEnv(t)
	t.Setenv("API_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to fail with COOKIE_SECURE=false and API_ENV=production, got nil error")
	}
}

func TestLoad_AllowsSecureCookieOutsideLocal(t *testing.T) {
	setMinimalRequiredEnv(t)
	t.Setenv("API_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load() to succeed with COOKIE_SECURE=true, got error: %v", err)
	}
	if !cfg.Cookie.Secure {
		t.Fatal("expected cfg.Cookie.Secure to be true")
	}
}

func TestLoad_AllowsInsecureCookieInLocal(t *testing.T) {
	setMinimalRequiredEnv(t)
	t.Setenv("API_ENV", "local")
	t.Setenv("COOKIE_SECURE", "false")

	_, err := Load()
	if err != nil {
		t.Fatalf("expected Load() to succeed with API_ENV=local regardless of COOKIE_SECURE, got error: %v", err)
	}
}

// TestLoad_RequiresBridgeInternalKeys covers platform-security-hardening
// task 0.4: the old single INTERNAL_API_KEY (with its "changeme"
// fallback) is gone — each bridge's key is its own required env var, in
// every environment (matching wa-web/tg-web's own unconditional startup
// checks), with no fallback value of any kind.
func TestLoad_RequiresBridgeInternalKeys(t *testing.T) {
	cases := []struct {
		name       string
		waWebKey   string
		tgWebKey   string
		wantErrStr string
	}{
		{"missing wa-web key", "", "tg-web-test-key", "WA_WEB_INTERNAL_KEY"},
		{"missing tg-web key", "wa-web-test-key", "", "TG_WEB_INTERNAL_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
			t.Setenv("TOKEN_ENCRYPTION_KEY", "dGhpc2lzYTMyYnl0ZWJhc2U2NGVuY29kZWRrZXkh")
			t.Setenv("WA_WEB_INTERNAL_KEY", tc.waWebKey)
			t.Setenv("TG_WEB_INTERNAL_KEY", tc.tgWebKey)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected Load() to fail when %s is empty", tc.wantErrStr)
			}
			if !strings.Contains(err.Error(), tc.wantErrStr) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tc.wantErrStr)
			}
		})
	}
}

func TestLoad_SucceedsWithDistinctBridgeKeys(t *testing.T) {
	setMinimalRequiredEnv(t)
	t.Setenv("API_ENV", "local")

	_, err := Load()
	if err != nil {
		t.Fatalf("expected Load() to succeed with both bridge keys set, got error: %v", err)
	}
}
