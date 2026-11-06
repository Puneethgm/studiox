package identity

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// totpIssuer names the authenticator-app entry (e.g. "1herosocial.ai (jane@…)").
const totpIssuer = "1herosocial.ai"

// totpSkew allows a code valid up to one 30-second period on either side
// of now — the "±30s window" in the design doc, matching how every real
// authenticator app's own clock drift tolerance works.
const totpSkew = 1

// generateTOTPSecret returns a fresh base32 TOTP secret and the
// otpauth:// URI an authenticator app (or a QR code rendered from it)
// uses to add the account — platform-security-hardening task 2.2.
func generateTOTPSecret(accountName string) (secret, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp secret: %w", err)
	}
	return key.Secret(), key.String(), nil
}

// validateTOTPCode checks code against secret at time now, within
// totpSkew periods either side. step is the matched period's index —
// callers must reject a step <= the last one accepted for this user
// (replay prevention; see Repo.MarkTOTPStepUsed) even though the code
// itself validates.
func validateTOTPCode(code, secret string, now time.Time) (ok bool, step int64, err error) {
	opts := totp.ValidateOpts{Period: 30, Skew: totpSkew, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1}
	// ValidateCustom doesn't return which step matched, so probe each
	// candidate step in the allowed skew window ourselves (totpSkew is
	// always small — at most a handful of iterations).
	// #nosec G115 -- opts.Period/opts.Skew are the hardcoded literals 30/1
	// set a few lines above, never derived from input; uint->int64
	// overflow is impossible for values this small.
	currentStep := now.Unix() / int64(opts.Period)
	for s := currentStep - int64(opts.Skew); s <= currentStep+int64(opts.Skew); s++ { // #nosec G115 -- opts.Skew is the hardcoded literal 1, never derived from input
		candidateTime := time.Unix(s*int64(opts.Period), 0) // #nosec G115 -- same hardcoded Period constant
		matched, verr := totp.ValidateCustom(code, secret, candidateTime, totp.ValidateOpts{Period: opts.Period, Digits: opts.Digits, Algorithm: opts.Algorithm})
		if verr != nil {
			return false, 0, fmt.Errorf("validate totp code: %w", verr)
		}
		if matched {
			return true, s, nil
		}
	}
	return false, 0, nil
}

// recoveryCodeAlphabet/Length mirror GenerateTempPassword's choices
// (password.go) — no look-alike characters, printed in a fixed-width
// font on the enrollment screen.
const recoveryCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const recoveryCodeLength = 10
const recoveryCodeCount = 10

// generateRecoveryCodes returns recoveryCodeCount fresh, random,
// single-use recovery codes — shown to the user exactly once (task 2.2),
// then only ever stored as bcrypt hashes (see Repo.CreateRecoveryCodes).
func generateRecoveryCodes() ([]string, error) {
	codes := make([]string, recoveryCodeCount)
	max := big.NewInt(int64(len(recoveryCodeAlphabet)))
	for i := range codes {
		buf := make([]byte, recoveryCodeLength)
		for j := range buf {
			n, err := rand.Int(rand.Reader, max)
			if err != nil {
				return nil, fmt.Errorf("generate recovery code: %w", err)
			}
			buf[j] = recoveryCodeAlphabet[n.Int64()]
		}
		// Hyphenated in two halves (e.g. ABCDE-FGHJK) purely for
		// readability when copying/typing — doesn't change the entropy.
		codes[i] = string(buf[:5]) + "-" + string(buf[5:])
	}
	return codes, nil
}

// normalizeRecoveryCode strips formatting (hyphens, spaces) and
// upper-cases, so a code is hashed and compared the same way regardless
// of how the user typed or pasted it back in.
func normalizeRecoveryCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}
