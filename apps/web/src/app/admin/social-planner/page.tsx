import SocialPlannerClient from '@/components/SocialPlannerClient';

export default function SuperAdminSocialPlannerPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-white">Social Planner & Scheduler</h1>
          <p className="text-xs text-zinc-400 font-semibold mt-0.5">Aggregate AI-driven ad campaign builder and global schedule grid</p>
        </div>
      </div>
      <SocialPlannerClient studioId="global" />
    </div>
  );
}
