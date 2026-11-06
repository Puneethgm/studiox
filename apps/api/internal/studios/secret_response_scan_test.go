package studios

import (
	"reflect"
	"regexp"
	"testing"
)

// secretShapedFieldName flags a struct field that looks like it carries
// raw secret material (a key, token, password, or secret value) rather
// than a safe derivative of one (a boolean presence flag, a status, or a
// non-reversible fingerprint/suffix-style display value).
var secretShapedFieldName = regexp.MustCompile(`(?i)(api)?key$|secret$|token$|password$|suffix$`)

// secretResponseFieldAllowlist lists field names that match
// secretShapedFieldName's pattern but are known-safe: a presence flag, a
// status enum, or a fingerprint that is itself the safe display value a
// raw secret was replaced with (platform-security-hardening task 0.2).
var secretResponseFieldAllowlist = map[string]bool{
	"HasAPIKey":      true,
	"KeyStatus":      true,
	"KeyFingerprint": true,
}

// secretBearingResponseTypes are HTTP handler response structs that
// represent a secret-bearing resource (a provider API key, an
// integration credential, etc). Add a new type here whenever a handler
// is introduced that reports on a secret's status — this is the
// regression guard task 0.3 asks for.
var secretBearingResponseTypes = []any{
	aiProviderResponse{},
}

// findRawSecretFields reports every field on v whose name matches
// secretShapedFieldName and is not in secretResponseFieldAllowlist —
// i.e. a field that looks like it might carry (or once carried, as
// keySuffix() did) a raw or partially-reversible secret value.
func findRawSecretFields(v any) []string {
	t := reflect.TypeOf(v)
	var violations []string
	for i := 0; i < t.NumField(); i++ {
		name := t.Field(i).Name
		if secretShapedFieldName.MatchString(name) && !secretResponseFieldAllowlist[name] {
			violations = append(violations, name)
		}
	}
	return violations
}

// TestSecretBearingResponses_NeverExposeRawSecretFields is the
// regression guard for platform-security-hardening task 0.3: no
// secret-bearing HTTP response type may carry a field shaped like a raw
// or partially-reversible secret value.
func TestSecretBearingResponses_NeverExposeRawSecretFields(t *testing.T) {
	for _, v := range secretBearingResponseTypes {
		if violations := findRawSecretFields(v); len(violations) > 0 {
			t.Errorf("%T exposes secret-shaped field(s) not in the allowlist: %v", v, violations)
		}
	}
}

// TestFindRawSecretFields_CatchesReintroducedSuffixReveal proves the
// checker itself would have caught the exact regression this task
// exists to prevent: keySuffix()'s prior "KeySuffix" field.
func TestFindRawSecretFields_CatchesReintroducedSuffixReveal(t *testing.T) {
	type regressedResponse struct {
		Provider  string
		HasAPIKey bool
		KeySuffix string // the removed field, reintroduced on purpose for this test
	}
	violations := findRawSecretFields(regressedResponse{})
	if len(violations) == 0 {
		t.Fatal("expected findRawSecretFields to flag KeySuffix, found no violations")
	}
	found := false
	for _, v := range violations {
		if v == "KeySuffix" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected violations to include KeySuffix, got %v", violations)
	}
}
