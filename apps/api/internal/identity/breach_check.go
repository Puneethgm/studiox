package identity

import (
	"bufio"
	"context"
	"crypto/sha1" // #nosec G505 -- required by the HaveIBeenPwned Pwned Passwords k-anonymity API protocol, see IsBreached below
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BreachChecker answers whether a password appears in a known-breach
// corpus (task 4.2). Behind an interface so tests use a fake rather
// than hitting a real external API (HaveIBeenPwned) on every run — see
// HIBPBreachChecker for the real implementation and
// fakeBreachChecker in the test file for the fixture used instead.
type BreachChecker interface {
	IsBreached(ctx context.Context, password string) (bool, error)
}

// HIBPBreachChecker calls the real HaveIBeenPwned Pwned Passwords
// k-anonymity API: only the first 5 hex chars of the password's SHA-1
// hash are ever sent over the network, never the password or its full
// hash, so the API itself never learns the actual password.
type HIBPBreachChecker struct {
	client *http.Client
}

func NewHIBPBreachChecker() *HIBPBreachChecker {
	return &HIBPBreachChecker{client: &http.Client{Timeout: 3 * time.Second}}
}

func (c *HIBPBreachChecker) IsBreached(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password)) // #nosec G401 G505 -- HaveIBeenPwned Pwned Passwords k-anonymity API requires SHA-1 by protocol; only a 5-char hash prefix is ever sent, never the password
	hash := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := hash[:5], hash[5:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.pwnedpasswords.com/range/"+prefix, nil)
	if err != nil {
		return false, fmt.Errorf("build HIBP request: %w", err)
	}
	// Explicitly opt OUT of HIBP's "padding" response variant (extra
	// decoy suffixes mixed in) to keep response parsing simple — padding
	// only protects against an attacker observing response SIZE on the
	// wire, which this server-side call isn't exposed to anyway.
	req.Header.Set("Add-Padding", "false")

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("call HIBP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HIBP returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(parts[0], suffix) {
			if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && n > 0 {
				return true, nil
			}
			return true, nil
		}
	}
	return false, scanner.Err()
}
