// Package guards checks a generated AI reply against the deterministic
// rules ai-reply-flow-llm-design doc §4.8 specifies (price/offer/claim/
// language/length/links/PII) before it reaches a customer — design.md's
// "rule-based first" decision: every check here is a plain string/regex
// comparison against the studio's own data pack, never an LLM call.
package guards

import (
	"regexp"
	"strings"
)

// DataPack is the subset of a studio's facts the guards need to validate
// a reply against — a narrower view than the full prompt data pack
// (internal/messaging's Plan/Offer), built fresh per reply so a guard
// check never trusts anything the model claimed about itself.
type DataPack struct {
	// ValidPrices are every price string ("S$99.00", "$99") that's
	// legitimately mentionable right now — from active plans and active
	// offers only. A price in the reply that isn't in this list fails
	// the price/offer/claim check.
	ValidPrices []string
	// ApprovedLinks are every URL that's legitimately mentionable
	// (payment links, booking links). A URL in the reply not in this
	// list fails the links check.
	ApprovedLinks []string
	// MaxLength bounds reply length in characters. 0 means no limit.
	MaxLength int
	// ExpectedLanguage, if set, is compared against the structured
	// reply's own Language field (not re-detected from text — the
	// structured call already classifies this, see CheckLanguage).
	ExpectedLanguage string
	// KnownCustomerFields are PII values already legitimately on file
	// for this customer (e.g. {"phone": "+6591234567"}) — a reply may
	// repeat these back, but inventing a value for a field NOT in this
	// map fails the PII check. See CheckPII's doc comment for the
	// (deliberately narrow) heuristic this uses.
	KnownCustomerFields map[string]string
}

// Violation is one guard check's specific failure — enough detail to
// drive both the repair-call prompt and a human-readable log line.
type Violation struct {
	Check  string // "price", "language", "length", "links", "pii"
	Detail string
}

// Result is one guard pass's outcome — Passed is true only when
// Violations is empty.
type Result struct {
	Passed     bool
	Violations []Violation
}

func passResult() Result { return Result{Passed: true} }

func failResult(violations ...Violation) Result {
	return Result{Passed: false, Violations: violations}
}

// priceRe matches a dollar-sign price token: "$99", "S$99.50", "$1,200".
// Deliberately only SGD-style ($/S$ prefix) since that's this codebase's
// only currency (see bifrost.Reply.CostUSD's own doc note that SGD is
// the only currency plans/offers are priced in) — a reply mentioning a
// price in another currency/format isn't checked by this guard and would
// need a broader pattern if that ever changes.
var priceRe = regexp.MustCompile(`S?\$[0-9][0-9,]*(?:\.[0-9]{1,2})?`)

// CheckPrices fails if reply mentions a price-looking token that isn't
// in pack.ValidPrices — doc §4.8's price/offer/claim check. Comparison
// is exact-string (normalized for whitespace) rather than numeric, since
// "the same number formatted differently" is itself often the actual
// bug (a stale cached price, a currency mismatch) this check exists to
// catch.
func CheckPrices(reply string, pack DataPack) Result {
	matches := priceRe.FindAllString(reply, -1)
	if len(matches) == 0 {
		return passResult()
	}
	valid := make(map[string]bool, len(pack.ValidPrices))
	for _, p := range pack.ValidPrices {
		valid[normalizePrice(p)] = true
	}
	var violations []Violation
	seen := make(map[string]bool)
	for _, m := range matches {
		norm := normalizePrice(m)
		if valid[norm] || seen[norm] {
			continue
		}
		seen[norm] = true
		violations = append(violations, Violation{
			Check:  "price",
			Detail: "reply mentions price " + m + " which is not an active plan or offer price",
		})
	}
	if len(violations) == 0 {
		return passResult()
	}
	return failResult(violations...)
}

func normalizePrice(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	return s
}

// CheckLength fails if reply exceeds pack.MaxLength characters (0 means
// unbounded — never fails).
func CheckLength(reply string, pack DataPack) Result {
	if pack.MaxLength <= 0 {
		return passResult()
	}
	if len(reply) <= pack.MaxLength {
		return passResult()
	}
	return failResult(Violation{
		Check:  "length",
		Detail: "reply is longer than the configured maximum",
	})
}

// urlRe matches an http(s) URL — intentionally simple (this checks
// against a known allow-list, not validating URL syntax).
var urlRe = regexp.MustCompile(`https?://[^\s)]+`)

// CheckLinks fails if reply contains a URL not in pack.ApprovedLinks.
func CheckLinks(reply string, pack DataPack) Result {
	matches := urlRe.FindAllString(reply, -1)
	if len(matches) == 0 {
		return passResult()
	}
	approved := make(map[string]bool, len(pack.ApprovedLinks))
	for _, l := range pack.ApprovedLinks {
		approved[strings.TrimRight(l, "/")] = true
	}
	var violations []Violation
	for _, m := range matches {
		trimmed := strings.TrimRight(strings.TrimRight(m, ".,);"), "/")
		if approved[trimmed] {
			continue
		}
		violations = append(violations, Violation{
			Check:  "links",
			Detail: "reply contains a link that isn't an approved payment/booking link: " + m,
		})
	}
	if len(violations) == 0 {
		return passResult()
	}
	return failResult(violations...)
}

// CheckLanguage fails if replyLanguage (the structured call's own
// classification — see internal/messaging.StructuredReply.Language) is
// set, pack.ExpectedLanguage is set, and they don't match. Deliberately
// does NOT re-detect language from the reply text itself — trusting a
// second heuristic layered on top of the model's own classification
// would just be two guesses instead of one; ExpectedLanguage should come
// from the customer's own message language, a fact already known
// upstream, not inferred here.
func CheckLanguage(replyLanguage string, pack DataPack) Result {
	if pack.ExpectedLanguage == "" || replyLanguage == "" {
		return passResult()
	}
	if strings.EqualFold(replyLanguage, pack.ExpectedLanguage) {
		return passResult()
	}
	return failResult(Violation{
		Check:  "language",
		Detail: "reply language (" + replyLanguage + ") does not match the expected language (" + pack.ExpectedLanguage + ")",
	})
}

// Check runs every deterministic guard (price, length, links, language,
// PII) and aggregates their violations — the single entry point
// ai_worker.go's send path (via checkAndRepair) calls.
func Check(reply, replyLanguage string, pack DataPack) Result {
	var all []Violation
	for _, r := range []Result{
		CheckPrices(reply, pack),
		CheckLength(reply, pack),
		CheckLinks(reply, pack),
		CheckLanguage(replyLanguage, pack),
		CheckPII(reply, pack),
	} {
		all = append(all, r.Violations...)
	}
	if len(all) == 0 {
		return passResult()
	}
	return failResult(all...)
}
