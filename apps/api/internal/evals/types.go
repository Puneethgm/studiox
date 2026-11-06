// Package evals implements the case-set eval framework from
// ai-reply-flow-llm-design (doc §8): a fixed set of real conversations,
// graded both by deterministic code checks and a model-judge call, run
// on demand via cmd/eval-replies before a reply-pipeline change ships.
package evals

// StudioContext is the minimal studio data pack fields a case can assert
// against — not the full studios.Studio type, so fixtures stay small and
// portable rather than coupled to the live schema.
type StudioContext struct {
	Name    string            `json:"name"`
	Offers  []string          `json:"offers,omitempty"`
	Plans   map[string]string `json:"plans,omitempty"` // plan name -> price, e.g. "Monthly": "$99"
	Hours   string            `json:"hours,omitempty"`
}

// PriorTurn is one earlier message in the conversation, for cases that
// need multi-turn context.
type PriorTurn struct {
	Role string `json:"role"` // "customer" or "ai"
	Text string `json:"text"`
}

// Expected is what a correct reply for this case must and must not do.
// Each field is optional — a case only sets the ones relevant to what
// it's testing.
type Expected struct {
	// MustMentionPrice, if set, is a price string (e.g. "$99") the reply
	// must contain when it references pricing at all.
	MustMentionPrice string `json:"must_mention_price,omitempty"`
	// MustNotMentionPrices lists price strings that would be WRONG for
	// this case — e.g. a different studio's price, or a stale one.
	MustNotMentionPrices []string `json:"must_not_mention_prices,omitempty"`
	// MustOfferHandoff is true when the correct behavior is NOT sending
	// an AI reply at all (gate should fire / guard should hand off).
	MustOfferHandoff bool `json:"must_offer_handoff,omitempty"`
	// Language is the expected reply language (ISO-ish short code or
	// name, e.g. "en", "ms") — empty means "don't check."
	Language string `json:"language,omitempty"`
	// Notes is free text for a human reviewer, not machine-checked.
	Notes string `json:"notes,omitempty"`
}

// Case is one fixture conversation in the eval case set.
type Case struct {
	ID              string        `json:"id"`
	Studio          StudioContext `json:"studio"`
	PriorTurns      []PriorTurn   `json:"prior_turns,omitempty"`
	CustomerMessage string        `json:"customer_message"`
	Expected        Expected      `json:"expected"`
}
