export const BILLING_CYCLES = [
  { value: 'monthly', label: 'Monthly' },
  { value: 'daily', label: 'Daily (testing)' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'biweekly', label: 'Every 2 Weeks' },
  { value: 'every_4_weeks', label: 'Every 4 Weeks' },
  { value: 'every_6_weeks', label: 'Every 6 Weeks' },
  { value: 'every_2_months', label: 'Every 2 Months' },
  { value: 'quarterly', label: 'Quarterly' },
  { value: 'semi_annually', label: 'Every 6 Months' },
  { value: 'yearly', label: 'Yearly' },
  { value: 'custom', label: 'Custom' },
  { value: 'one_time', label: 'One-time' },
] as const;

// Stripe's own accepted recurring interval units — matches
// internal/platform/billing.ValidIntervals on the backend.
export const BILLING_INTERVAL_UNITS = [
  { value: 'day', label: 'Day(s)' },
  { value: 'week', label: 'Week(s)' },
  { value: 'month', label: 'Month(s)' },
  { value: 'year', label: 'Year(s)' },
] as const;

const UNIT_WORD: Record<string, string> = { day: 'Day', week: 'Week', month: 'Month', year: 'Year' };
const UNIT_ABBREV: Record<string, string> = { day: 'd', week: 'wk', month: 'mo', year: 'yr' };

const SHORT_SUFFIX: Record<string, string> = {
  monthly: '/mo',
  daily: '/day',
  weekly: '/wk',
  biweekly: '/2wk',
  every_4_weeks: '/4wk',
  every_6_weeks: '/6wk',
  every_2_months: '/2mo',
  quarterly: '/qtr',
  semi_annually: '/6mo',
  yearly: '/yr',
};

/** Human-readable label for a plan's cadence, e.g. "Every 5 Weeks" for a custom plan. */
export function cycleLabel(billingCycle: string, billingInterval?: string, billingIntervalCount?: number): string {
  if (billingCycle === 'custom') {
    const count = billingIntervalCount && billingIntervalCount > 0 ? billingIntervalCount : 1;
    const word = UNIT_WORD[billingInterval || 'month'] || 'Month';
    return `Every ${count} ${word}${count === 1 ? '' : 's'}`;
  }
  return BILLING_CYCLES.find((c) => c.value === billingCycle)?.label ?? billingCycle;
}

/** Short "/mo"-style suffix for a price tag. Empty string for one-time plans. */
export function cycleShortSuffix(billingCycle: string, billingInterval?: string, billingIntervalCount?: number): string {
  if (billingCycle === 'one_time') return '';
  if (billingCycle === 'custom') {
    const count = billingIntervalCount && billingIntervalCount > 0 ? billingIntervalCount : 1;
    return `/${count}${UNIT_ABBREV[billingInterval || 'month'] || 'mo'}`;
  }
  return SHORT_SUFFIX[billingCycle] || '/mo';
}
