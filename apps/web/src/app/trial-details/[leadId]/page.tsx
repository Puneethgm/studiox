import { Suspense } from 'react';
import { headers } from 'next/headers';
import { TrialPaymentPage } from './TrialPaymentPage';

export default async function TrialDetailsPage({
  params,
}: {
  params: Promise<{ leadId: string }>;
}) {
  const { leadId } = await params;
  const nonce = (await headers()).get('x-nonce') ?? undefined;
  return (
    <Suspense>
      <TrialPaymentPage leadId={leadId} nonce={nonce} />
    </Suspense>
  );
}
