import { TrialPaymentPage } from '../[leadId]/TrialPaymentPage';

// A studio-scoped, no-lead-required preview of the trial payment page —
// renders exactly what a customer would see (real branding, real saved
// layout, real Stripe card fields) but with the Pay button disabled, so it's
// safe to open without a real lead attached. Linked from the builder page.
export default function TrialDetailsPreviewPage() {
  return <TrialPaymentPage preview />;
}
