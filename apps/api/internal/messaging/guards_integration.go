package messaging

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/llmgateway"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging/guards"
	"github.com/projectx/api/internal/messaging/prompts"
)

// defaultMaxReplyLength bounds a generated reply's length (guards.CheckLength).
// The design doc (reply-guards spec) requires a "configured maximum" but
// doesn't specify a value or a per-studio setting for it, so this is a
// deliberate, documented policy choice rather than an invented one: a
// WhatsApp text message's own hard limit is 4096 characters, and a studio's
// AI reply is a short chat message, not a document — 1600 characters
// (roughly a long paragraph) is used as an "something has clearly gone
// wrong" ceiling, not an attempt to police normal reply length. Revisit if
// this proves too tight/loose once real traffic is observed.
const defaultMaxReplyLength = 1600

// buildGuardDataPack turns the facts already fetched for the reply
// prompt (plans, offers, the lead's known contact details) into
// guards.DataPack — the same facts, reshaped for checking instead of
// prompting. expectedLanguage comes from ClassifyIntent's piggybacked
// language detection (see studios.ClassifyIntent); knownCustomerFields
// comes from the lead's own on-file contact details, so guards.CheckPII
// can tell "echoing back what we already have on file" apart from "the
// model invented this."
func buildGuardDataPack(plans []Plan, offers []Offer, expectedLanguage string, knownCustomerFields map[string]string) guards.DataPack {
	pack := guards.DataPack{
		ExpectedLanguage:    expectedLanguage,
		MaxLength:           defaultMaxReplyLength,
		KnownCustomerFields: knownCustomerFields,
	}
	for _, p := range plans {
		pack.ValidPrices = append(pack.ValidPrices, formatSGD(p.PriceSGD))
		if p.PaymentLink != "" {
			pack.ApprovedLinks = append(pack.ApprovedLinks, p.PaymentLink)
		}
	}
	for _, o := range offers {
		pack.ValidPrices = append(pack.ValidPrices, formatSGD(o.PriceSGD))
	}
	return pack
}

// knownCustomerFieldsFromLead builds guards.DataPack.KnownCustomerFields
// from a lead's own on-file contact details. lead may be nil (e.g. a
// conversation not yet linked to a lead) — returns an empty map, which
// guards.CheckPII treats as "nothing on file to legitimately echo."
func knownCustomerFieldsFromLead(lead *leads.Lead) map[string]string {
	fields := map[string]string{}
	if lead == nil {
		return fields
	}
	if lead.Email != "" {
		fields["email"] = lead.Email
	}
	if lead.Phone != "" {
		fields["phone"] = lead.Phone
	}
	return fields
}

func formatSGD(cents int) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100.0)
}

// checkAndRepair runs the deterministic guards (guards.Check) against
// structured's reply; on failure, makes exactly one repair call asking
// the model to fix the specific violation(s), then checks the repaired
// reply too. Returns (finalReply, ok) — ok is false only when both the
// original and the repaired reply fail guards, meaning the caller must
// hand off to staff instead of sending anything (doc §4.8's bounded
// repair-then-handoff, task 4.4).
func (w *AIWorker) checkAndRepair(ctx context.Context, studioID uuid.UUID, structured StructuredReply, pack guards.DataPack) (finalReply string, ok bool) {
	result := guards.Check(structured.Reply, structured.Language, pack)
	if result.Passed {
		return structured.Reply, true
	}

	violationDetails := make([]string, len(result.Violations))
	for i, v := range result.Violations {
		violationDetails[i] = v.Detail
	}
	if w.log != nil {
		w.log.Warn("ai reply failed guard checks, attempting one repair call",
			"studio_id", studioID, "violations", violationDetails)
	}

	repairPrompt, err := prompts.Render(prompts.Repair, prompts.RepairData{
		PreviousReply: structured.Reply,
		Violations:    violationDetails,
	})
	if err != nil {
		// A broken template file is a programmer error, not a runtime
		// condition — fail closed to handoff rather than send an
		// unrepaired, already-failing reply.
		if w.log != nil {
			w.log.Error("render repair prompt failed", "err", err)
		}
		return "", false
	}

	repaired, _, err := llmgateway.CompleteStructured[StructuredReply](
		ctx, w.llmGateway, llmgateway.FeatureCustomerReply, studioID, repairPrompt,
		StructuredReplySchemaName, StructuredReplyJSONSchema,
	)
	if err != nil {
		if w.log != nil {
			w.log.Warn("repair call failed, handing off", "studio_id", studioID, "err", err)
		}
		return "", false
	}

	repairResult := guards.Check(repaired.Reply, repaired.Language, pack)
	if !repairResult.Passed {
		if w.log != nil {
			repairViolations := make([]string, len(repairResult.Violations))
			for i, v := range repairResult.Violations {
				repairViolations[i] = v.Detail
			}
			w.log.Warn("repaired reply still fails guards, handing off",
				"studio_id", studioID, "violations", repairViolations)
		}
		return "", false
	}
	return repaired.Reply, true
}
