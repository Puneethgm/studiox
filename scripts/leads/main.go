// Standalone leads-data tool for Project-X.
//
// Three subcommands:
//   populate   insert N realistic leads (weighted across the funnel)
//   progress   walk existing leads forward (new → contacted → trial → member|dropped)
//   reset      delete all source='seed' leads on a campaign
//
// Reads .env at the repo root (POSTGRES_* vars). Connects directly to Postgres
// — no API server involved, so seeded leads do NOT enqueue Sheets exports.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case "populate":
		runPopulate(args)
	case "progress":
		runProgress(args)
	case "reset":
		runReset(args)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`Project-X leads tool.

Usage:
  go run . populate --studio=<slug> --campaign=<slug> [--count=80]
      Insert N realistic leads spread over the last 30 days, with a
      weighted status mix (new 40 / contacted 25 / trial 15 / member 12 / dropped 8).
      Leads are tagged source='seed' so they can be cleanly removed later.

  go run . progress --studio=<slug> --campaign=<slug> [--count=20]
      Walk existing non-terminal leads forward in the funnel:
        new → contacted → trial_booked → member|dropped
      Picks the oldest-updated leads first; about a third stay where they are
      to keep the funnel realistic.

  go run . reset --studio=<slug> --campaign=<slug>
      Delete every source='seed' lead on this campaign. Real submissions
      (source='public_form') are untouched.

Examples:
  go run . populate --studio=fawaz --campaign=spring-season-onki --count=80
  go run . progress --studio=fawaz --campaign=spring-season-onki --count=15
  go run . reset    --studio=fawaz --campaign=spring-season-onki

Database: reads .env (POSTGRES_*). Run from this directory or the repo root.`)
}

// ----- populate -----

func runPopulate(args []string) {
	fs := flag.NewFlagSet("populate", flag.ExitOnError)
	studioSlug := fs.String("studio", "", "studio slug (required)")
	campaignSlug := fs.String("campaign", "", "campaign slug (required)")
	count := fs.Int("count", 80, "number of leads to insert")
	_ = fs.Parse(args)

	if *studioSlug == "" || *campaignSlug == "" {
		fmt.Fprintln(os.Stderr, "--studio and --campaign are required")
		fs.Usage()
		os.Exit(2)
	}
	if *count <= 0 {
		fmt.Fprintln(os.Stderr, "--count must be > 0")
		os.Exit(2)
	}

	ctx := context.Background()
	pool := mustConnect(ctx)
	defer pool.Close()

	studioID, campaignID, plans := resolveCampaign(ctx, pool, *studioSlug, *campaignSlug)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tally := map[string]int{}
	failed := 0

	for i := 0; i < *count; i++ {
		l := generateLead(rng, plans)
		// Random created_at across the last 30 days, with a slight skew toward recent.
		offset := time.Duration(rng.Int63n(int64(30 * 24 * time.Hour)))
		// 25% chance of being "this week" — gives the dashboard something fresh.
		if rng.Intn(4) == 0 {
			offset = time.Duration(rng.Int63n(int64(7 * 24 * time.Hour)))
		}
		created := time.Now().Add(-offset)

		_, err := pool.Exec(ctx, `
			INSERT INTO leads (studio_id, campaign_id, name, email, phone, fitness_plan,
			                   goals, source, status, notes, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'seed',$8,$9,$10,$10)
		`,
			studioID, campaignID,
			l.name, l.email, l.phone, l.plan, l.goals,
			l.status, l.notes, created,
		)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  ! lead %d (%s): %v\n", i, l.email, err)
			continue
		}
		tally[l.status]++
	}

	fmt.Printf("\n✓ inserted %d leads into %s / %s\n",
		*count-failed, *studioSlug, *campaignSlug)
	printTally(tally)
	if failed > 0 {
		fmt.Printf("  %d failed\n", failed)
	}
}

// ----- progress -----

func runProgress(args []string) {
	fs := flag.NewFlagSet("progress", flag.ExitOnError)
	studioSlug := fs.String("studio", "", "studio slug (required)")
	campaignSlug := fs.String("campaign", "", "campaign slug (required)")
	count := fs.Int("count", 20, "number of leads to consider for progression")
	_ = fs.Parse(args)

	if *studioSlug == "" || *campaignSlug == "" {
		fmt.Fprintln(os.Stderr, "--studio and --campaign are required")
		fs.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	pool := mustConnect(ctx)
	defer pool.Close()

	studioID, campaignID, _ := resolveCampaign(ctx, pool, *studioSlug, *campaignSlug)

	// Pull the oldest-updated non-terminal leads — these are the ones most
	// "due" for a follow-up touch.
	rows, err := pool.Query(ctx, `
		SELECT id, status FROM leads
		WHERE studio_id = $1 AND campaign_id = $2
		  AND status IN ('new','contacted','trial_booked')
		ORDER BY updated_at ASC
		LIMIT $3
	`, studioID, campaignID, *count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query candidates: %v\n", err)
		os.Exit(1)
	}

	type candidate struct {
		id     uuid.UUID
		status string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.status); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			continue
		}
		candidates = append(candidates, c)
	}
	rows.Close()

	if len(candidates) == 0 {
		fmt.Println("nothing to progress — no leads in new/contacted/trial_booked status.")
		return
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	transitions := map[string]int{} // "new->contacted": N
	stayed := 0

	for _, c := range candidates {
		next := nextStatus(rng, c.status)
		if next == c.status {
			stayed++
			continue
		}
		note := noteFor(rng, next)
		_, err := pool.Exec(ctx, `
			UPDATE leads
			SET status = $2,
			    notes = CASE
			      WHEN notes = '' THEN $3
			      ELSE notes || E'\n— ' || $3
			    END,
			    updated_at = now()
			WHERE id = $1
		`, c.id, next, note)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ! update %s: %v\n", c.id, err)
			continue
		}
		transitions[c.status+" → "+next]++
	}

	fmt.Printf("\n✓ examined %d candidates in %s / %s\n", len(candidates), *studioSlug, *campaignSlug)
	keys := make([]string, 0, len(transitions))
	for k := range transitions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("    %-30s %d\n", k, transitions[k])
	}
	if stayed > 0 {
		fmt.Printf("    %-30s %d\n", "(stayed put)", stayed)
	}
}

// ----- reset -----

func runReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	studioSlug := fs.String("studio", "", "studio slug (required)")
	campaignSlug := fs.String("campaign", "", "campaign slug (required)")
	_ = fs.Parse(args)

	if *studioSlug == "" || *campaignSlug == "" {
		fmt.Fprintln(os.Stderr, "--studio and --campaign are required")
		fs.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	pool := mustConnect(ctx)
	defer pool.Close()

	studioID, campaignID, _ := resolveCampaign(ctx, pool, *studioSlug, *campaignSlug)

	tag, err := pool.Exec(ctx, `
		DELETE FROM leads
		WHERE studio_id = $1 AND campaign_id = $2 AND source = 'seed'
	`, studioID, campaignID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "delete: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ deleted %d seeded leads from %s / %s (real submissions untouched)\n",
		tag.RowsAffected(), *studioSlug, *campaignSlug)
}

// ----- helpers -----

func printTally(tally map[string]int) {
	order := []string{"new", "contacted", "trial_booked", "member", "dropped"}
	for _, k := range order {
		if v, ok := tally[k]; ok {
			fmt.Printf("    %-14s %d\n", k, v)
		}
	}
}
