import { serverFetch } from '@/lib/auth';
import { PageHeader } from '@/components/ui/PageHeader';
import type { TrialPageLayout, Studio } from '@/lib/types';
import { TrialPageBuilder } from './TrialPageBuilder';

export default async function TrialPageBuilderPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const [resp, studio] = await Promise.all([
    serverFetch<TrialPageLayout>(`/api/v1/me/studios/${studioId}/trial-page-layout`),
    serverFetch<Studio>(`/api/v1/me/studios/${studioId}`),
  ]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Trial Payment Page"
        description="Design the page customers see when they click your WhatsApp trial payment link — drag, resize, and edit anything. The card payment fields stay Stripe-secured; everything else is fully yours."
      />
      <TrialPageBuilder
        studioId={studioId}
        studioSlug={studio?.slug ?? ''}
        initialBlocks={resp?.blocks ?? null}
        initialBackground={resp?.background ?? null}
      />
    </div>
  );
}
