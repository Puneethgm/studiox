import { notFound } from 'next/navigation';
import { Sparkles, CalendarRange } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { fetchPublicCampaign, fetchPublicStudio } from '@/lib/public';
import { BookingClient } from './BookingClient';

const noiseSvgDataUri =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160' viewBox='0 0 160 160'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='160' height='160' filter='url(%23n)' opacity='0.38'/%3E%3C/svg%3E";

export default async function BookingPage({
  params,
  searchParams,
}: {
  params: Promise<{ studioSlug: string; campaignSlug: string }>;
  searchParams: Promise<{ leadId?: string }>;
}) {
  const { studioSlug, campaignSlug } = await params;
  const { leadId } = await searchParams;

  if (!leadId) notFound();

  const [studio, campaign] = await Promise.all([
    fetchPublicStudio(studioSlug),
    fetchPublicCampaign(studioSlug, campaignSlug),
  ]);
  
  if (!studio || !campaign) notFound();

  const brand = studio.brandColor || '#7c3aed';

  return (
    <main
      className="relative min-h-screen overflow-hidden px-4 py-16 sm:px-6 lg:px-8"
      style={{
        backgroundImage: `radial-gradient(circle at 0% 0%, ${brand}18 0%, transparent 40%), radial-gradient(circle at 100% 100%, ${brand}18 0%, transparent 40%), linear-gradient(rgba(238, 240, 245, 0.55), rgba(238, 240, 245, 0.55)), url('/admin-bg-light.png')`,
        backgroundSize: 'cover',
        backgroundPosition: 'center top',
        backgroundAttachment: 'fixed',
      }}
    >
      <div aria-hidden className="pointer-events-none absolute inset-0 opacity-60">
        <div className="absolute -left-[10%] top-[18%] h-[42%] w-[42%] rounded-full bg-brand-500/10 blur-[120px]" />
        <div className="absolute -right-[12%] bottom-[10%] h-[42%] w-[42%] rounded-full bg-sky-400/10 blur-[120px]" />
        <div
          className="absolute inset-0 opacity-[0.04]"
          style={{
            backgroundImage: `url("${noiseSvgDataUri}")`,
            backgroundRepeat: 'repeat',
            backgroundSize: '160px 160px',
          }}
        />
      </div>
      
      <div className="mx-auto max-w-2xl animate-slide-up">
        <div className="mb-10 text-center">
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
          <h1 className="mt-4 text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
            Book Your Trial Slot
          </h1>
          <p className="mx-auto mt-2 max-w-md text-slate-500 dark:text-slate-400">
            Select an available date and time slot below to schedule your class.
          </p>
        </div>

        <Card 
          title={
            <div className="flex items-center gap-2 text-lg font-bold">
              <CalendarRange className="h-5 w-5" style={{ color: brand }} />
              <span>Schedule Appointment</span>
            </div>
          } 
          elevated 
          className="overflow-hidden border-none shadow-2xl shadow-slate-200/50 dark:shadow-none"
        >
          <BookingClient
            leadId={leadId}
            brandColor={brand}
            studioName={studio.name}
            campaignName={campaign.name}
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
