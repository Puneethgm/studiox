'use client';

import { useRouter } from 'next/navigation';
import type { Studio } from '@/lib/types';

export function StudioSelector({ studios, selectedId }: { studios: Studio[]; selectedId: string }) {
  const router = useRouter();

  return (
    <select
      value={selectedId}
      onChange={(e) => {
        const id = e.target.value;
        router.push(id ? `/admin/analytics?studioId=${id}` : '/admin/analytics');
      }}
      className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-800 dark:text-white"
    >
      <option value="">All Studios (Global)</option>
      {studios.map((s) => (
        <option key={s.id} value={s.id}>
          {s.name}
        </option>
      ))}
    </select>
  );
}
