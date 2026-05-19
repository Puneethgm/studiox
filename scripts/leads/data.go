package main

import (
	"fmt"
	"math/rand"
	"strings"
)

// ----- name / contact pools -----

var firstNames = []string{
	"Aisha", "Alex", "Amar", "Arjun", "Bina", "Carlos", "Daniel", "Elena", "Farah", "Grace",
	"Hassan", "Imani", "Ivy", "Jasper", "Kira", "Liam", "Maya", "Noah", "Olivia", "Priya",
	"Quentin", "Rahul", "Sofia", "Tariq", "Una", "Vihaan", "Wei", "Xian", "Yara", "Zane",
	"Mei", "Hiro", "Sasha", "Diego", "Anya", "Jonas", "Leo", "Nora", "Ravi", "Tessa",
}

var lastNames = []string{
	"Kapoor", "Lim", "Tan", "Wilson", "Patel", "Garcia", "Chen", "Khan", "Reddy", "Rodriguez",
	"Nguyen", "Smith", "Park", "Singh", "Brown", "Ali", "Lee", "Sharma", "Kim", "Davis",
	"Costa", "Yamamoto", "Aziz", "Müller", "Rossi", "Ahmed", "Goh", "Iyer",
}

var emailDomains = []string{"gmail.com", "outlook.com", "yahoo.com", "icloud.com", "proton.me"}

var goalsPool = []string{
	"Lose 5kg before summer.",
	"Build strength after a long break.",
	"Train for first half marathon.",
	"Recover from a knee injury — looking for low-impact options.",
	"Improve flexibility and core strength.",
	"Get back in shape post-pregnancy.",
	"Reduce stress with regular yoga.",
	"Cross-train for football season.",
	"Just looking to try something new.",
	"Want consistency — currently sporadic.",
	"Tone up before a wedding in October.",
	"Mostly want a workout buddy / community.",
	"", "", "", // some empty
}

// ----- status distribution -----

type statusBucket struct {
	status string
	weight int
	notes  []string
}

// Weights sum to 100 for clarity. Keeps a healthy spread across the funnel.
var buckets = []statusBucket{
	{"new", 40, []string{""}},
	{"contacted", 25, []string{
		"Called Mon, asked about trial pricing.",
		"Sent intro email, awaiting reply.",
		"Reached out via WhatsApp — interested.",
		"Spoke briefly, asked us to follow up next week.",
		"Left a voicemail.",
	}},
	{"trial_booked", 15, []string{
		"Trial booked Friday 7pm.",
		"Trial Sunday morning class.",
		"Booked intro session with Emma.",
		"Confirmed trial — bringing a friend.",
	}},
	{"member", 12, []string{
		"Signed up for monthly plan.",
		"Joined 6-month package.",
		"Annual membership.",
		"Family plan, 2 adults.",
	}},
	{"dropped", 8, []string{
		"No response after 3 attempts.",
		"Said budget too high right now.",
		"Moved out of city.",
		"Decided to go with another studio.",
	}},
}

// noteBank lets the `progress` command pick a contextually-appropriate note
// when advancing a lead into a new status.
var noteBank = map[string][]string{
	"contacted":    buckets[1].notes,
	"trial_booked": buckets[2].notes,
	"member":       buckets[3].notes,
	"dropped":      buckets[4].notes,
}

// ----- generators -----

type lead struct {
	name, email, phone, plan, goals, status, notes string
}

func generateLead(rng *rand.Rand, plans []string) lead {
	fn := firstNames[rng.Intn(len(firstNames))]
	ln := lastNames[rng.Intn(len(lastNames))]
	name := fn + " " + ln

	suffix := ""
	if rng.Intn(2) == 0 {
		suffix = fmt.Sprintf("%d", 10+rng.Intn(89))
	}
	email := strings.ToLower(fmt.Sprintf("%s.%s%s@%s",
		fn, ln, suffix, emailDomains[rng.Intn(len(emailDomains))]))

	// Singapore-style mobile, e.g. "+65 9123 4567" — passes the API's regex.
	phone := fmt.Sprintf("+65 9%03d %04d", rng.Intn(1000), rng.Intn(10000))

	plan := plans[rng.Intn(len(plans))]
	goals := goalsPool[rng.Intn(len(goalsPool))]

	bucket := pickBucket(rng)
	notes := ""
	if len(bucket.notes) > 0 && bucket.notes[0] != "" {
		notes = bucket.notes[rng.Intn(len(bucket.notes))]
	}

	return lead{
		name: name, email: email, phone: phone,
		plan: plan, goals: goals,
		status: bucket.status, notes: notes,
	}
}

func pickBucket(rng *rand.Rand) statusBucket {
	total := 0
	for _, b := range buckets {
		total += b.weight
	}
	roll := rng.Intn(total)
	acc := 0
	for _, b := range buckets {
		acc += b.weight
		if roll < acc {
			return b
		}
	}
	return buckets[0]
}

// nextStatus implements a weighted Markov-chain transition for the `progress`
// command. Returning the same status means "leave this one alone."
func nextStatus(rng *rand.Rand, current string) string {
	roll := rng.Intn(100)
	switch current {
	case "new":
		// 60% advance, 10% drop, 30% stay
		if roll < 60 {
			return "contacted"
		}
		if roll < 70 {
			return "dropped"
		}
	case "contacted":
		// 50% advance, 20% drop, 30% stay
		if roll < 50 {
			return "trial_booked"
		}
		if roll < 70 {
			return "dropped"
		}
	case "trial_booked":
		// 60% convert, 15% drop, 25% stay
		if roll < 60 {
			return "member"
		}
		if roll < 75 {
			return "dropped"
		}
	}
	return current
}

// noteFor picks a randomized note for a destination status, used by `progress`
// to make the audit trail look real.
func noteFor(rng *rand.Rand, status string) string {
	notes, ok := noteBank[status]
	if !ok || len(notes) == 0 {
		return ""
	}
	return notes[rng.Intn(len(notes))]
}
