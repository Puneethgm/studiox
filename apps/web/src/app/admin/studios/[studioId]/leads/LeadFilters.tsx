'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { Select } from '@/components/ui/Select';
import { LEAD_STATUSES, LEAD_STATUS_LABELS } from '@/lib/types';

export function LeadFilters({ status }: { status?: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();

  function setStatus(next: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (next) params.set('status', next);
    else params.delete('status');
    router.push(`?${params.toString()}`);
  }

  return (
    <div className="flex items-center gap-3">
      <label htmlFor="status-filter" className="text-sm font-medium text-slate-700 dark:text-slate-300">
        Status
      </label>
      <Select
        id="status-filter"
        className="w-48"
        value={status ?? ''}
        onChange={(e) => setStatus(e.target.value)}
      >
        <option value="">All statuses</option>
        {LEAD_STATUSES.map((s) => (
          <option key={s} value={s}>
            {LEAD_STATUS_LABELS[s]}
          </option>
        ))}
      </Select>
    </div>
  );
}
