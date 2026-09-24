// Package billing holds cadence logic shared by the studios and messaging
// packages for turning a plans.billing_cycle value into a real Stripe
// recurring-price shape, so membership checkout actually charges on the
// cadence a plan advertises instead of always billing monthly.
package billing

import "fmt"

// ValidCycles is the allowlist of billing_cycle values plans may be created
// or updated with. "custom" additionally requires a valid billing_interval +
// billing_interval_count on the plan — see Resolve.
var ValidCycles = map[string]bool{
	"monthly":        true,
	"yearly":         true,
	"one_time":       true,
	"daily":          true,
	"weekly":         true,
	"biweekly":       true,
	"every_4_weeks":  true,
	"every_6_weeks":  true,
	"quarterly":      true,
	"semi_annually":  true,
	"every_2_months": true,
	"custom":         true,
}

// ValidIntervals is the allowlist of Stripe recurring interval units a
// "custom" plan cycle may use.
var ValidIntervals = map[string]bool{
	"day":   true,
	"week":  true,
	"month": true,
	"year":  true,
}

// maxIntervalCount mirrors Stripe's own limits per interval unit
// (https://stripe.com/docs/api/prices/create#create_price-recurring-interval_count):
// day up to 365, week up to 52, month up to 12, year up to 3.
var maxIntervalCount = map[string]int64{
	"day":   365,
	"week":  52,
	"month": 12,
	"year":  3,
}

// RecurringForCycle maps a preset plans.billing_cycle value to the Stripe
// recurring shape needed to bill it. recurring=false means a one-time charge
// (Checkout mode "payment"); interval/intervalCount are meaningless in that
// case. It does not handle "custom" — use Resolve for that.
func RecurringForCycle(cycle string) (interval string, intervalCount int64, recurring bool) {
	switch cycle {
	case "one_time":
		return "", 0, false
	case "yearly":
		return "year", 1, true
	case "semi_annually":
		return "month", 6, true
	case "quarterly":
		return "month", 3, true
	case "every_2_months":
		return "month", 2, true
	case "daily":
		return "day", 1, true
	case "weekly":
		return "week", 1, true
	case "biweekly":
		return "week", 2, true
	case "every_4_weeks":
		return "week", 4, true
	case "every_6_weeks":
		return "week", 6, true
	case "monthly":
		fallthrough
	default:
		return "month", 1, true
	}
}

// ValidateCustomInterval checks a "custom" plan's stored interval/count
// against Stripe's own accepted units and per-unit maximums. Called at the
// plan create/update boundary so a bad custom cadence is rejected before it
// can ever reach checkout.
func ValidateCustomInterval(interval string, intervalCount int64) error {
	if !ValidIntervals[interval] {
		return fmt.Errorf("billingInterval must be one of day, week, month, year")
	}
	if intervalCount < 1 {
		return fmt.Errorf("billingIntervalCount must be at least 1")
	}
	if max := maxIntervalCount[interval]; intervalCount > max {
		return fmt.Errorf("billingIntervalCount for %s billing cannot exceed %d", interval, max)
	}
	return nil
}

// Resolve returns the Stripe recurring shape for a plan's billing_cycle,
// consulting customInterval/customIntervalCount only when cycle == "custom".
// Assumes the custom interval was already validated at the plan boundary via
// ValidateCustomInterval — an invalid stored value falls back to monthly
// rather than breaking checkout outright.
func Resolve(cycle, customInterval string, customIntervalCount int64) (interval string, intervalCount int64, recurring bool) {
	if cycle == "custom" {
		if err := ValidateCustomInterval(customInterval, customIntervalCount); err != nil {
			return "month", 1, true
		}
		return customInterval, customIntervalCount, true
	}
	return RecurringForCycle(cycle)
}
