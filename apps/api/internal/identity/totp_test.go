package identity

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPSecret_ProducesValidatableSecretAndURI(t *testing.T) {
	secret, uri, err := generateTOTPSecret("jane@example.com")
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	if secret == "" {
		t.Fatal("expected a non-empty secret")
	}
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("uri = %q, want it to start with otpauth://totp/", uri)
	}
	if !strings.Contains(uri, totpIssuer) {
		t.Errorf("uri = %q, want it to mention the issuer %q", uri, totpIssuer)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	ok, _, err := validateTOTPCode(code, secret, time.Now())
	if err != nil {
		t.Fatalf("validateTOTPCode: %v", err)
	}
	if !ok {
		t.Error("expected a code generated from the same secret/time to validate")
	}
}

func TestValidateTOTPCode_AcceptsWithinSkewWindowRejectsOutside(t *testing.T) {
	secret, _, err := generateTOTPSecret("jane@example.com")
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	now := time.Now()

	// 25 seconds in the past is still within the same or adjacent 30s
	// period — should validate.
	nearPast := now.Add(-25 * time.Second)
	code, err := totp.GenerateCode(secret, nearPast)
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	ok, _, err := validateTOTPCode(code, secret, now)
	if err != nil {
		t.Fatalf("validateTOTPCode: %v", err)
	}
	if !ok {
		t.Error("expected a code from 25s ago to validate within the ±30s skew window")
	}

	// Far outside the skew window (several periods away) must be rejected.
	farPast := now.Add(-5 * time.Minute)
	staleCode, err := totp.GenerateCode(secret, farPast)
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	ok, _, err = validateTOTPCode(staleCode, secret, now)
	if err != nil {
		t.Fatalf("validateTOTPCode: %v", err)
	}
	if ok {
		t.Error("expected a code from 5 minutes ago to be rejected as outside the skew window")
	}
}

func TestValidateTOTPCode_WrongCodeRejected(t *testing.T) {
	secret, _, err := generateTOTPSecret("jane@example.com")
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	ok, _, err := validateTOTPCode("000000", secret, time.Now())
	if err != nil {
		t.Fatalf("validateTOTPCode: %v", err)
	}
	// Astronomically unlikely to be the real code, but don't flake if it is.
	realCode, _ := totp.GenerateCode(secret, time.Now())
	if "000000" != realCode && ok {
		t.Error("expected an arbitrary wrong code to be rejected")
	}
}

func TestGenerateRecoveryCodes_TenUniqueCodes(t *testing.T) {
	codes, err := generateRecoveryCodes()
	if err != nil {
		t.Fatalf("generateRecoveryCodes: %v", err)
	}
	if len(codes) != recoveryCodeCount {
		t.Fatalf("got %d codes, want %d", len(codes), recoveryCodeCount)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate recovery code generated: %q", c)
		}
		seen[c] = true
		if !strings.Contains(c, "-") {
			t.Errorf("expected code %q to contain a formatting hyphen", c)
		}
	}
}

func TestNormalizeRecoveryCode(t *testing.T) {
	cases := map[string]string{
		"ABCDE-FGHJK": "ABCDEFGHJK",
		"abcde-fghjk": "ABCDEFGHJK",
		" ABCDE FGHJK ": "ABCDEFGHJK",
		"ABCDEFGHJK":   "ABCDEFGHJK",
	}
	for in, want := range cases {
		if got := normalizeRecoveryCode(in); got != want {
			t.Errorf("normalizeRecoveryCode(%q) = %q, want %q", in, got, want)
		}
	}
}
