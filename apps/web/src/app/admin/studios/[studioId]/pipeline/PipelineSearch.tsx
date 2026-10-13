'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Search, X } from 'lucide-react';

// Same debounced URL-param search as the Leads list page's LeadFilters —
// the Pipeline board had no search at all, so finding a specific lead among
// a large column meant scrolling through every card by eye.
export function PipelineSearch({ search }: { search?: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [value, setValue] = useState(search ?? '');

  useEffect(() => {
    setValue(search ?? '');
  }, [search]);

  useEffect(() => {
    const current = searchParams.get('search') ?? '';
    if (value === current) return;
    const handler = setTimeout(() => {
      const params = new URLSearchParams(searchParams.toString());
      if (value.trim()) {
        params.set('search', value.trim());
      } else {
        params.delete('search');
      }
      router.push(`?${params.toString()}`);
    }, 350);
    return () => clearTimeout(handler);
  }, [value, searchParams, router]);

  return (
    <div className="relative w-full max-w-[240px] shrink-0">
      <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400" />
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Search name, phone, email…"
        className="w-full rounded-full border border-zinc-200 bg-white/70 py-1.5 pl-8 pr-7 text-xs font-semibold text-zinc-800 placeholder:text-zinc-400 focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-900/70 dark:text-zinc-100"
      />
      {value && (
        <button
          onClick={() => setValue('')}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  );
}
