package identity

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// MinPasswordLength is platform-security-hardening task 4.1's new
// minimum — 12 characters, no complexity rule (length alone is the
// stronger, more usable control per the design doc). Shared constant so
// changePassword/resetPassword/studio-admin-creation enforce the exact
// same boundary rather than three independently-maintained literals.
const MinPasswordLength = 12

func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// tempPasswordAlphabet omits look-alike characters (0/O, 1/l/I) so a password
// read out of an email is hard to mistype.
const tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// tempPasswordLength gives about 93 bits of entropy.
const tempPasswordLength = 16

// GenerateTempPassword returns a random one-time password for a newly created
// account (studio teammate or studio admin). It replaces the old shared fixed
// default: every new account now gets its own, and must_reset_password is still
// set so it only works until the first password change.
func GenerateTempPassword() (string, error) {
	out := make([]byte, tempPasswordLength)
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate temp password: %w", err)
		}
		out[i] = tempPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}
