import SocialPlannerClient from '@/components/SocialPlannerClient';

export default async function StudioSocialPlannerPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Social Planner</h1>
          <p className="text-xs text-zinc-500 dark:text-zinc-400 font-semibold mt-0.5">AI-driven ad creation and schedule management for your studio location</p>
        </div>
      </div>
      <SocialPlannerClient studioId={studioId} />
    </div>
  );
}
