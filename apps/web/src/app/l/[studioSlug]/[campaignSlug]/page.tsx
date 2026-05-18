import { notFound } from 'next/navigation';
import { Sparkles } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { fetchPublicCampaign, fetchPublicStudio } from '@/lib/public';
import { LeadForm } from './form';

export default async function CampaignFormPage({
  params,
}: {
  params: Promise<{ studioSlug: string; campaignSlug: string }>;
}) {
  const { studioSlug, campaignSlug } = await params;
  const [studio, campaign] = await Promise.all([
    fetchPublicStudio(studioSlug),
    fetchPublicCampaign(studioSlug, campaignSlug),
  ]);
  if (!studio || !campaign) notFound();

  // Per-studio branding: drive the gradient + accent colors from the studio's
  // brand_color via inline style on the wrapper. Fully isolated to this route.
  const brand = studio.brandColor;

  return (
    <main
      className="min-h-screen px-4 py-16 sm:px-6 lg:px-8"
      style={{
        background: `radial-gradient(circle at 0% 0%, ${brand}15 0%, transparent 40%), radial-gradient(circle at 100% 100%, ${brand}15 0%, transparent 40%), #f8fafc`,
      }}
    >
      <div className="mx-auto max-w-xl animate-slide-up">
        <div className="mb-12 text-center">
          <div
            className="mx-auto mb-6 grid h-20 w-20 place-items-center overflow-hidden rounded-3xl text-2xl font-black text-white shadow-2xl ring-8 ring-white dark:ring-slate-900"
            style={{ background: brand }}
          >
            {studio.logoUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={studio.logoUrl} alt={studio.name} className="h-20 w-20 object-cover" />
            ) : (
              studio.name.slice(0, 2).toUpperCase()
            )}
          </div>
          <div className="inline-block rounded-full bg-slate-100 px-4 py-1.5 text-[10px] font-bold uppercase tracking-[0.2em] text-slate-500 dark:bg-slate-800 dark:text-slate-400">
            {studio.name}
          </div>
          <h1 className="mt-4 text-4xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-5xl">
            {campaign.name}
          </h1>
          {campaign.description && (
            <p className="mx-auto mt-4 max-w-md text-lg leading-relaxed text-slate-600 dark:text-slate-400">
              {campaign.description}
            </p>
          )}
        </div>

        <Card 
          title={<span className="text-lg font-bold">Registration</span>} 
          subtitle="Complete the form below to get started."
          elevated 
          className="overflow-hidden border-none shadow-2xl shadow-slate-200/50 dark:shadow-none"
        >
          <LeadForm
            studioSlug={studio.slug}
            campaignSlug={campaign.slug}
            fitnessPlans={campaign.fitnessPlans}
            brandColor={brand}
          />
        </Card>

        <div className="mt-8 flex items-center justify-center gap-2 text-xs font-medium text-slate-400">
          <Sparkles className="h-4 w-4 text-brand-500" />
          <span>Powered by 1herosocial.ai</span>
        </div>
      </div>
    </main>
  );
}
