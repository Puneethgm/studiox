import { Suspense } from 'react';
import { TrialPaymentPage } from '../../trial-details/[leadId]/TrialPaymentPage';

// Same branded page as the trial-details flow (same component, same studio-
// designed layout) — the ?planId= query param is what switches it from
// "pay the trial fee" to "pay for this membership plan". See
// buildPlanCheckoutBody (messaging/service.go), which links here instead of
// a hosted Stripe Checkout redirect.
export default async function PlanDetailsPage({
  params,
}: {
  params: Promise<{ leadId: string }>;
}) {
  const { leadId } = await params;
  return (
    <Suspense>
      <TrialPaymentPage leadId={leadId} />
    </Suspense>
  );
}
