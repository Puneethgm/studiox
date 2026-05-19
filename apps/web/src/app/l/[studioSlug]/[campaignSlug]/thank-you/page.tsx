import { Card } from '@/components/ui/Card';
import { fetchPublicStudio } from '@/lib/public';

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
      className="grid min-h-screen place-items-center px-4"
      style={{
        background: `linear-gradient(160deg, ${brand}10 0%, #ffffff 50%, ${brand}18 100%)`,
      }}
    >
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
