import { serverFetch } from '@/lib/auth';
import type { Studio } from '@/lib/types';
import { TemplatesClient } from './TemplatesClient';

export const dynamic = 'force-dynamic';

export default async function TemplatesPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const studio = await serverFetch<Studio>(`/api/v1/me/studios/${studioId}`);

  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2 leading-relaxed">
        Customize the automated messages sent to leads. Leave a template blank to use the default text.
      </div>
      <TemplatesClient studio={studio} />
    </div>
  );
}
