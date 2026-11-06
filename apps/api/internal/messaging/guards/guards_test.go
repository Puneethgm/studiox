package guards

import "testing"

func TestCheckPrices(t *testing.T) {
	pack := DataPack{ValidPrices: []string{"$99", "S$49.00"}}

	cases := []struct {
		name   string
		reply  string
		passed bool
	}{
		{"matching price passes", "The monthly plan is $99.", true},
		{"matching offer price passes", "The intro offer is S$49.00.", true},
		{"wrong price fails", "That's S$899/year.", false},
		{"no price mentioned passes", "Sure, I can help with that!", true},
		{"same price repeated only flagged once", "It's $899, yes $899 confirmed.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckPrices(c.reply, pack)
			if got.Passed != c.passed {
				t.Errorf("CheckPrices(%q) passed = %v, want %v (violations: %+v)", c.reply, got.Passed, c.passed, got.Violations)
			}
		})
	}
}

func TestCheckPrices_DeduplicatesRepeatedViolation(t *testing.T) {
	pack := DataPack{ValidPrices: []string{"$99"}}
	got := CheckPrices("It's $899, yes $899 confirmed.", pack)
	if len(got.Violations) != 1 {
		t.Errorf("expected exactly 1 violation for a repeated wrong price, got %d: %+v", len(got.Violations), got.Violations)
	}
}

func TestCheckLength(t *testing.T) {
	pack := DataPack{MaxLength: 10}
	if !CheckLength("short", pack).Passed {
		t.Error("a reply within the limit should pass")
	}
	if CheckLength("this is way too long", pack).Passed {
		t.Error("a reply over the limit should fail")
	}
	noLimit := DataPack{MaxLength: 0}
	if !CheckLength("this is way too long but no limit is set", noLimit).Passed {
		t.Error("MaxLength=0 should mean unbounded")
	}
}

func TestCheckLinks(t *testing.T) {
	pack := DataPack{ApprovedLinks: []string{"https://pay.example.com/monthly"}}

	if !CheckLinks("Pay here: https://pay.example.com/monthly", pack).Passed {
		t.Error("an approved link should pass")
	}
	if CheckLinks("Pay here: https://evil.example.com/phish", pack).Passed {
		t.Error("an unapproved link should fail")
	}
	if !CheckLinks("No links here at all.", pack).Passed {
		t.Error("no links mentioned should pass")
	}
	// Trailing punctuation/slash shouldn't cause a false mismatch.
	if !CheckLinks("See https://pay.example.com/monthly/.", pack).Passed {
		t.Error("an approved link with trailing punctuation should still pass")
	}
}

func TestCheckLanguage(t *testing.T) {
	cases := []struct {
		name             string
		replyLanguage    string
		expectedLanguage string
		passed           bool
	}{
		{"matching language passes", "en", "en", true},
		{"mismatched language fails", "en", "ms", false},
		{"no expectation set passes", "en", "", true},
		{"no reply language classified passes", "", "ms", true},
		{"case-insensitive match passes", "EN", "en", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pack := DataPack{ExpectedLanguage: c.expectedLanguage}
			got := CheckLanguage(c.replyLanguage, pack)
			if got.Passed != c.passed {
				t.Errorf("CheckLanguage(%q, expected=%q) passed = %v, want %v", c.replyLanguage, c.expectedLanguage, got.Passed, c.passed)
			}
		})
	}
}

func TestCheck_AggregatesAllDeterministicChecks(t *testing.T) {
	pack := DataPack{
		ValidPrices:      []string{"$99"},
		MaxLength:        1000,
		ApprovedLinks:    []string{"https://pay.example.com/ok"},
		ExpectedLanguage: "en",
	}

	// Clean reply passes everything.
	if got := Check("It's $99/month. See https://pay.example.com/ok", "en", pack); !got.Passed {
		t.Errorf("expected a clean reply to pass all checks, got violations: %+v", got.Violations)
	}

	// Reply with a bad price AND a bad language should report both.
	got := Check("It's $899/month.", "ms", pack)
	if got.Passed {
		t.Fatal("expected this reply to fail")
	}
	if len(got.Violations) != 2 {
		t.Errorf("expected 2 violations (price + language), got %d: %+v", len(got.Violations), got.Violations)
	}
}

// TestCheck_CatchesInventedPII proves CheckPII is actually wired into the
// single production entry point (Check), not just independently passing
// its own standalone tests — the exact gap the ai-reply-flow-llm-design
// audit flagged: a reply inventing an NRIC the customer never gave must
// fail through Check, the same aggregator ai_worker.go's checkAndRepair
// calls, not just through CheckPII called in isolation.
func TestCheck_CatchesInventedPII(t *testing.T) {
	pack := DataPack{
		ValidPrices: []string{"$99"},
		// No NRIC on file for this customer — pack.KnownCustomerFields
		// deliberately omits one, so any NRIC-shaped value in the reply
		// is necessarily invented.
		KnownCustomerFields: map[string]string{"email": "jane@example.com"},
	}

	// A clean reply with no PII-shaped content still passes.
	if got := Check("It's $99/month, see you soon!", "", pack); !got.Passed {
		t.Errorf("expected a clean reply with no PII to pass, got violations: %+v", got.Violations)
	}

	// A reply inventing an NRIC not on file must fail via Check (not just
	// CheckPII called directly) — this is what proves the production
	// aggregator actually runs the PII check.
	got := Check("Sure, I've noted your NRIC as S1234567D for our records.", "", pack)
	if got.Passed {
		t.Fatal("expected Check to reject a reply containing an invented NRIC the customer never gave")
	}
	foundPII := false
	for _, v := range got.Violations {
		if v.Check == "pii" {
			foundPII = true
		}
	}
	if !foundPII {
		t.Errorf("expected a 'pii' violation among Check's results, got: %+v", got.Violations)
	}
}
