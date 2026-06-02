import SocialPlannerClient from '@/components/SocialPlannerClient';
import { Megaphone } from 'lucide-react';

export default async function StudioSocialPlannerPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2">
        AI-driven ad creation and schedule management for your studio location.
      </div>
      <SocialPlannerClient studioId={studioId} />
    </div>
  );
}
