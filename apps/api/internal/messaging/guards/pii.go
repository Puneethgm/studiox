package guards

import "regexp"

// piiPatterns are regexes for PII shapes worth checking at all — a
// narrow, deliberately conservative starting set (design.md's Open
// Questions: "refined once the eval case set surfaces real examples").
// Each one matches a shape that, if present in a reply, must correspond
// to a value already in pack.KnownCustomerFields — never a value the
// model invented.
var piiPatterns = map[string]*regexp.Regexp{
	// NRIC/FIN (Singapore): letter, 7 digits, letter — e.g. S1234567D.
	"nric": regexp.MustCompile(`\b[STFG]\d{7}[A-Z]\b`),
	// A date written as DD/MM/YYYY or DD-MM-YYYY — the shape a date of
	// birth would take if the model invented one (see dev-pii-not-on-file
	// eval case).
	"date": regexp.MustCompile(`\b\d{1,2}[/-]\d{1,2}[/-]\d{4}\b`),
}

// CheckPII fails if reply contains a PII-shaped value that doesn't match
// any value already in pack.KnownCustomerFields — i.e. the model
// invented or guessed a value it was never actually given. Deliberately
// narrow and shape-based (NRIC, date-of-birth-like) rather than a broad
// PII classifier: a false positive here blocks a legitimate reply, so
// this only flags shapes specific enough to be worth checking at all.
// Called from Check (guards.go) alongside the other deterministic checks.
//
// Known false-positive risk, by design for now (design.md's Open
// Questions: "can be refined once the eval case set surfaces real
// examples"): the "date" pattern matches ANY DD/MM/YYYY-shaped value, not
// just a date of birth — a legitimate class-date or deadline mention in a
// reply (e.g. "your class is on 15/11/2026") will also match and, unless
// that exact string happens to be in KnownCustomerFields, fail this
// check. That's an intentional conservative starting point (fail toward
// a repair/handoff rather than risk leaking an invented PII value), but
// worth monitoring in production logs for how often it fires on
// legitimate dates rather than real invented PII.
func CheckPII(reply string, pack DataPack) Result {
	if len(pack.KnownCustomerFields) == 0 {
		// Nothing on file to compare against — matches dev-pii-not-on-file's
		// case: ANY date-shaped value here would be invented, since there's
		// no known field it could legitimately be echoing.
		for name, re := range piiPatterns {
			if re.MatchString(reply) {
				return failResult(Violation{
					Check:  "pii",
					Detail: "reply contains a " + name + "-shaped value but no customer PII is on file to legitimately echo back",
				})
			}
		}
		return passResult()
	}

	knownValues := make(map[string]bool, len(pack.KnownCustomerFields))
	for _, v := range pack.KnownCustomerFields {
		knownValues[v] = true
	}

	var violations []Violation
	for name, re := range piiPatterns {
		for _, m := range re.FindAllString(reply, -1) {
			if knownValues[m] {
				continue
			}
			violations = append(violations, Violation{
				Check:  "pii",
				Detail: "reply contains a " + name + "-shaped value (" + m + ") not found among the customer's known on-file data",
			})
		}
	}
	if len(violations) == 0 {
		return passResult()
	}
	return failResult(violations...)
}
