// Command glofox-attendance is a one-off diagnostic: it pulls every booking
// from Glofox for the configured branch, tallies how many classes each
// member has actually attended, and prints everyone who has hit the
// milestone threshold.
//
// Read-only — this sends nothing. It does not send a WhatsApp message, it
// does not write to the database. It only prints to stdout, for a human to
// review before any messaging feature is built on top of this.
//
// Glofox has no "list members" or paginated/filtered bookings endpoint —
// ListBookings returns every booking for the branch in one call, so this
// does one branch-wide fetch, then one GetMember call per qualifying user
// (not per booking) to resolve name/phone/email.
//
// Usage: cd apps/api && go run ./cmd/glofox-attendance
package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/platform/config"
)

const minClassesAttended = 5

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	gf := glofox.New(cfg.Glofox.APIKey, cfg.Glofox.APIToken, cfg.Glofox.BranchID)
	if gf == nil {
		os.Stderr.WriteString("glofox not configured — set GLOFOX_API_KEY, GLOFOX_API_TOKEN, GLOFOX_BRANCH_ID\n")
		os.Exit(1)
	}

	// No timeout: manual, operator-run command against an unpaginated
	// branch-wide endpoint that can return a lot of rows.
	ctx := context.Background()

	bookings, err := gf.ListBookings(ctx)
	if err != nil {
		os.Stderr.WriteString("list bookings: " + err.Error() + "\n")
		os.Exit(1)
	}

	attendedCount := make(map[string]int)
	lastKnownName := make(map[string]string)
	for _, b := range bookings {
		if b.UserID == "" {
			continue
		}
		lastKnownName[b.UserID] = b.UserName
		if b.Attended {
			attendedCount[b.UserID]++
		}
	}

	type qualifier struct {
		userID string
		count  int
	}
	var qualifiers []qualifier
	for userID, count := range attendedCount {
		if count >= minClassesAttended {
			qualifiers = append(qualifiers, qualifier{userID, count})
		}
	}
	sort.Slice(qualifiers, func(i, j int) bool { return qualifiers[i].count > qualifiers[j].count })

	fmt.Printf("Fetched %d bookings, %d distinct users, %d with %d+ attended classes.\n\n",
		len(bookings), len(lastKnownName), len(qualifiers), minClassesAttended)

	fmt.Println("Attended-class counts, everyone with at least one booking (for context):")
	var allUserIDs []string
	for userID := range lastKnownName {
		allUserIDs = append(allUserIDs, userID)
	}
	sort.Slice(allUserIDs, func(i, j int) bool { return attendedCount[allUserIDs[i]] > attendedCount[allUserIDs[j]] })
	for _, userID := range allUserIDs {
		fmt.Printf("  %-26s %-22s %d attended\n", userID, lastKnownName[userID], attendedCount[userID])
	}
	fmt.Println()

	if len(qualifiers) == 0 {
		fmt.Printf("No one has reached %d attended classes yet.\n", minClassesAttended)
		return
	}

	fmt.Printf("%-26s %-22s %-16s %-28s %s\n", "USER ID", "NAME", "PHONE", "EMAIL", "CLASSES ATTENDED")
	for _, q := range qualifiers {
		name := lastKnownName[q.userID]
		phone, email := "?", "?"
		member, err := gf.GetMember(ctx, q.userID)
		if err != nil {
			phone = "(lookup failed: " + err.Error() + ")"
		} else {
			if n := member.FirstName + " " + member.LastName; n != " " {
				name = n
			}
			phone = member.Phone
			email = member.ResolveEmail()
		}
		fmt.Printf("%-26s %-22s %-16s %-28s %d\n", q.userID, name, phone, email, q.count)
	}

	fmt.Println("\nNo WhatsApp message was sent — this is a dry-run report only.")
}
