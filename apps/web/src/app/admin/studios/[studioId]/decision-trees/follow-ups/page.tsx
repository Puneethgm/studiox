import { serverFetch } from '@/lib/auth';
import type { FollowupStep } from '@/lib/types';
import { PageHeader } from '@/components/ui/PageHeader';
import { DecisionTreesTabs } from '../DecisionTreesTabs';
import { FollowupsEditor } from './FollowupsEditor';

interface StepsResp {
  steps: FollowupStep[];
}

export default async function FollowUpsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const resp = await serverFetch<StepsResp>(
    `/api/v1/studios/${studioId}/messaging/followup-steps`,
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="Decision Trees"
        description="Configure branching reply flows for customer messages. The active tree is used automatically when a customer message arrives."
      />

      <DecisionTreesTabs studioId={studioId} />

      <FollowupsEditor studioId={studioId} initialSteps={resp?.steps ?? []} />
    </div>
  );
}
