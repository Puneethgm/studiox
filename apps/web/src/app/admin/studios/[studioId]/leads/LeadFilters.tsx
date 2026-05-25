'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { Select } from '@/components/ui/Select';
import { LEAD_STATUSES, LEAD_STATUS_LABELS } from '@/lib/types';
import { Search, X } from 'lucide-react';
import { useEffect, useState } from 'react';

export function LeadFilters({ status, search }: { status?: string; search?: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [searchValue, setSearchValue] = useState(search ?? '');

  // Keep state sync with external query changes (e.g. reset/browser history back)
  useEffect(() => {
    setSearchValue(search ?? '');
  }, [search]);

  // Debounce search input value
  useEffect(() => {
    const currentSearch = searchParams.get('search') ?? '';
    if (searchValue === currentSearch) return;

    const handler = setTimeout(() => {
      const params = new URLSearchParams(searchParams.toString());
      if (searchValue.trim()) {
        params.set('search', searchValue.trim());
      } else {
        params.delete('search');
      }
      // Reset to page 1 on search queries
      params.delete('page');
      router.push(`?${params.toString()}`);
    }, 350);

    return () => clearTimeout(handler);
  }, [searchValue, searchParams, router]);

  function setStatus(next: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (next) params.set('status', next);
    else params.delete('status');
    // Reset to page 1 on filter changes
    params.delete('page');
    router.push(`?${params.toString()}`);
  }

  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-3xl border border-white/20 bg-white/20 p-4 backdrop-blur-xl dark:border-white/5 dark:bg-neutral-900/20 shadow-sm">
      {/* Search Box */}
      <div className="relative flex-1 max-w-md">
        <Search className="absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
        <input
          type="text"
          placeholder="Search leads by name, email, or phone..."
          value={searchValue}
          onChange={(e) => setSearchValue(e.target.value)}
          className="w-full rounded-2xl border border-zinc-200/50 bg-white/50 py-2.5 pl-11 pr-10 text-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-zinc-900 dark:text-white"
        />
        {searchValue && (
          <button
            onClick={() => setSearchValue('')}
            className="absolute right-3.5 top-1/2 -translate-y-1/2 rounded-full p-0.5 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* Dropdown Filters */}
      <div className="flex items-center gap-3">
        <label htmlFor="status-filter" className="text-xs font-black uppercase tracking-wider text-zinc-400">
          Status:
        </label>
        <Select
          id="status-filter"
          className="w-44 rounded-2xl border border-zinc-200/50 bg-white/50 py-2 focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-xs font-bold"
          value={status ?? ''}
          onChange={(e) => setStatus(e.target.value)}
        >
          <option value="">All Statuses</option>
          {LEAD_STATUSES.map((s) => (
            <option key={s} value={s}>
              {LEAD_STATUS_LABELS[s]}
            </option>
          ))}
        </Select>
      </div>
    </div>
  );
}
