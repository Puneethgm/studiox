import { Database } from 'lucide-react';
import { serverFetch } from '@/lib/auth';
import type { Studio } from '@/lib/types';
import { KnowledgeBaseForm } from './KnowledgeBaseForm';

export default async function KnowledgeBasePage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const studio = await serverFetch<Studio>(`/api/v1/me/studios/${studioId}`);

  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2">
        Feed guidelines, prices, policies, and files directly to the AI Assistant.
      </div>
      <KnowledgeBaseForm studio={studio} />
    </div>
  );
}
