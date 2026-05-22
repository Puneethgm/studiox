import { Card } from '@/components/ui/Card';
import { fetchPublicStudio } from '@/lib/public';

const noiseSvgDataUri =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160' viewBox='0 0 160 160'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='160' height='160' filter='url(%23n)' opacity='0.38'/%3E%3C/svg%3E";

export default async function ThankYouPage({
  params,
}: {
  params: Promise<{ studioSlug: string; campaignSlug: string }>;
}) {
  const { studioSlug } = await params;
  const studio = await fetchPublicStudio(studioSlug);
  const brand = studio?.brandColor ?? '#7c3aed';

  return (
    <main
      className="relative grid min-h-screen place-items-center overflow-hidden px-4"
      style={{
        backgroundImage: `radial-gradient(circle at 0% 0%, ${brand}18 0%, transparent 40%), radial-gradient(circle at 100% 100%, ${brand}18 0%, transparent 40%), linear-gradient(rgba(238, 240, 245, 0.55), rgba(238, 240, 245, 0.55)), url('/admin-bg-light.png')`,
        backgroundSize: 'cover',
        backgroundPosition: 'center top',
        backgroundAttachment: 'fixed',
      }}
    >
      <div aria-hidden className="pointer-events-none absolute inset-0 opacity-60">
        <div className="absolute -left-[12%] top-[22%] h-[46%] w-[46%] rounded-full bg-brand-500/10 blur-[120px]" />
        <div className="absolute -right-[10%] bottom-[8%] h-[46%] w-[46%] rounded-full bg-sky-400/10 blur-[120px]" />
        <div
          className="absolute inset-0 opacity-[0.04]"
          style={{
            backgroundImage: `url("${noiseSvgDataUri}")`,
            backgroundRepeat: 'repeat',
            backgroundSize: '160px 160px',
          }}
        />
      </div>
      <div className="w-full max-w-md">
        <Card elevated>
          <div className="py-8 text-center">
            <div
              className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-full text-white shadow-md"
              style={{ background: brand }}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="h-7 w-7"
              >
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </div>
            <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Thanks!</h1>
            <p className="mt-2 text-slate-600">
              {studio?.name ?? 'The studio'} has received your details and will reach out shortly.
            </p>
          </div>
        </Card>
      </div>
    </main>
  );
}
