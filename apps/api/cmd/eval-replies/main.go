// Command eval-replies runs the AI reply pipeline against the eval case
// set (internal/evals/cases/) and writes a graded report — the on-demand
// tool from ai-reply-flow-llm-design's doc §8 "run before shipping a
// pipeline change" eval framework. Not a service: run it, read the
// report, done.
//
// Usage: cd apps/api && go run ./cmd/eval-replies [-split dev|heldout|all] [-out report.json]
//
// By default this calls the real `customer_reply` LLM route via
// llmgateway.CompleteStructured, using the exact same StructuredReply
// schema (internal/messaging.StructuredReply/StructuredReplyJSONSchema)
// the production ai_worker.go reply path uses as of task 3.2 — a minimal
// prompt built from each case's studio context, not yet the full
// production debounce/gates/guards/tools pipeline (those are wired in
// across Group 4-7's task groups, at which point this generator can call
// that pipeline's reusable entry point directly instead of building its
// own smaller prompt).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/evals"
	"github.com/projectx/api/internal/integrations/bifrost"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/integrations/llmgateway"
	"github.com/projectx/api/internal/messaging"
	"github.com/projectx/api/internal/messaging/guards"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/logger"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

func main() {
	splitFlag := flag.String("split", "all", `which case split to run: "dev", "heldout", or "all"`)
	outFlag := flag.String("out", "eval-report.json", "path to write the JSON report to")
	casesDirFlag := flag.String("cases", "internal/evals/cases", "path to the case-set directory (containing dev/ and heldout/)")
	flag.Parse()

	dev, heldout, err := evals.LoadCaseSetFromPath(*casesDirFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load case set:", err)
		os.Exit(1)
	}

	var cases []evals.Case
	switch *splitFlag {
	case "dev":
		cases = dev
	case "heldout":
		cases = heldout
	case "all":
		cases = append(append([]evals.Case{}, dev...), heldout...)
	default:
		fmt.Fprintf(os.Stderr, "unknown -split %q (want dev, heldout, or all)\n", *splitFlag)
		os.Exit(1)
	}
	if len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "no cases loaded for split", *splitFlag)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.TokenEncryptionKey == "" {
		fmt.Fprintln(os.Stderr, "TOKEN_ENCRYPTION_KEY must be set (platform AI provider keys are envelope-encrypted with it)")
		os.Exit(1)
	}
	cipher, err := secrets.New(cfg.TokenEncryptionKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init cipher:", err)
		os.Exit(1)
	}

	bfClient := bifrost.New()
	llmRepo := llm.NewRepo(pool, cipher)
	studiosRepo := studios.NewRepo(pool, cipher)
	gwRepo := llmgateway.NewRepo(pool)
	gw := llmgateway.New(bfClient, llmRepo, studiosRepo, gwRepo, log)

	generate := defaultGenerator(gw)
	report := evals.Run(ctx, cases, generate, nil)

	if err := evals.WriteJSON(report, *outFlag); err != nil {
		fmt.Fprintln(os.Stderr, "write report:", err)
		os.Exit(1)
	}

	fmt.Printf("eval run complete: %d pass, %d fail, %d error (of %d cases) — report written to %s\n",
		report.PassCount, report.FailCount, report.ErrorCount, len(cases), *outFlag)
	if report.FailCount > 0 || report.ErrorCount > 0 {
		os.Exit(1)
	}
}

// defaultGenerator builds a minimal prompt from a case's studio context
// and customer message, and calls the real customer_reply gateway route
// via CompleteStructured — the same structured call the production
// pipeline uses. See this file's package doc comment for why this isn't
// yet the full production reply pipeline (debounce/gates/guards/tools).
func defaultGenerator(gw *llmgateway.Client) evals.ReplyGenerator {
	return func(ctx context.Context, c evals.Case) (evals.Reply, error) {
		var b strings.Builder
		fmt.Fprintf(&b, "You are a helpful assistant for %s.\n", c.Studio.Name)
		if c.Studio.Hours != "" {
			fmt.Fprintf(&b, "Hours: %s\n", c.Studio.Hours)
		}
		if len(c.Studio.Plans) > 0 {
			b.WriteString("Plans:\n")
			for name, price := range c.Studio.Plans {
				fmt.Fprintf(&b, "- %s: %s\n", name, price)
			}
		}
		if len(c.Studio.Offers) > 0 {
			b.WriteString("Active offers:\n")
			for _, o := range c.Studio.Offers {
				fmt.Fprintf(&b, "- %s\n", o)
			}
		}
		for _, t := range c.PriorTurns {
			fmt.Fprintf(&b, "%s: %s\n", t.Role, t.Text)
		}
		fmt.Fprintf(&b, "customer: %s\n", c.CustomerMessage)
		b.WriteString("Reply as the studio's assistant, using only the facts above.")

		structured, _, err := llmgateway.CompleteStructured[messaging.StructuredReply](
			ctx, gw, llmgateway.FeatureCustomerReply, uuid.Nil, b.String(),
			messaging.StructuredReplySchemaName, messaging.StructuredReplyJSONSchema,
		)
		if err != nil {
			// Matches CompleteStructured's own "no route / every provider
			// failed or unparseable" contract — in an eval run with no
			// provider configured, that's a handoff, not a crash.
			return evals.Reply{Handoff: true}, nil
		}
		if structured.Action == "handoff" {
			return evals.Reply{Handoff: true}, nil
		}

		// Guard check (task 4.6) — no repair call here, unlike the
		// production path's checkAndRepair: this is the eval CLI's
		// prompt-only generator (see the package doc comment), not the
		// full pipeline, so a guard failure here is reported as a failing
		// case rather than retried.
		pack := guards.DataPack{ExpectedLanguage: ""}
		for _, price := range c.Studio.Plans {
			pack.ValidPrices = append(pack.ValidPrices, price)
		}
		if result := guards.Check(structured.Reply, structured.Language, pack); !result.Passed {
			return evals.Reply{Handoff: true}, nil
		}

		return evals.Reply{Text: structured.Reply}, nil
	}
}
