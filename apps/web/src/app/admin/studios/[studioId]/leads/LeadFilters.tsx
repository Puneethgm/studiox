'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { Select } from '@/components/ui/Select';
import { LEAD_STATUSES, LEAD_STATUS_LABELS, Campaign } from '@/lib/types';
import { Search, X, Calendar } from 'lucide-react';
import { useEffect, useState } from 'react';

export function LeadFilters({
  status,
  search,
  source,
  duration,
  startDate,
  endDate,
  sources = [],
  campaigns = [],
  campaignId,
}: {
  status?: string;
  search?: string;
  source?: string;
  duration?: string;
  startDate?: string;
  endDate?: string;
  sources?: string[];
  campaigns?: Campaign[];
  campaignId?: string;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [searchValue, setSearchValue] = useState(search ?? '');
  const [localStartDate, setLocalStartDate] = useState(startDate ?? '');
  const [localEndDate, setLocalEndDate] = useState(endDate ?? '');

  // Keep state sync with external query changes
  useEffect(() => {
    setSearchValue(search ?? '');
  }, [search]);

  useEffect(() => {
    setLocalStartDate(startDate ?? '');
  }, [startDate]);

  useEffect(() => {
    setLocalEndDate(endDate ?? '');
  }, [endDate]);

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
      params.delete('page');
      router.push(`?${params.toString()}`);
    }, 350);

    return () => clearTimeout(handler);
  }, [searchValue, searchParams, router]);

  function setStatus(next: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (next) params.set('status', next);
    else params.delete('status');
    params.delete('page');
    router.push(`?${params.toString()}`);
  }

  function setCampaign(next: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (next) params.set('campaignId', next);
    else params.delete('campaignId');
    params.delete('page');
    router.push(`?${params.toString()}`);
  }

  // Handle dynamic multi-source filtering
  function setSource(next: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (next) params.set('source', next);
    else params.delete('source');
    params.delete('page');
    router.push(`?${params.toString()}`);
  }

  function setDuration(next: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (next) {
      params.set('duration', next);
    } else {
      params.delete('duration');
    }
    
    // Clear custom dates if duration is not custom
    if (next !== 'custom') {
      params.delete('startDate');
      params.delete('endDate');
    }
    
    params.delete('page');
    router.push(`?${params.toString()}`);
  }

  // Handle custom date submission
  function handleCustomDateChange(start: string, end: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (start) params.set('startDate', start);
    else params.delete('startDate');
    if (end) params.set('endDate', end);
    else params.delete('endDate');
    params.delete('page');
    router.push(`?${params.toString()}`);
  }

  return (
    <div className="flex flex-col gap-4 rounded-3xl border border-white/20 bg-white/20 p-4 backdrop-blur-xl dark:border-white/5 dark:bg-neutral-900/20 shadow-sm">
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
        {/* Search Box */}
        <div className="relative flex-1 max-w-sm">
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
        <div className="flex flex-wrap items-center gap-4">
          {/* Campaign Filter */}
          <div className="flex items-center gap-2.5">
            <span className="text-xs font-black uppercase tracking-wider text-zinc-400">Campaign:</span>
            <Select
              className="w-36 rounded-2xl border border-zinc-200/50 bg-white/50 py-2 focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-xs font-bold"
              value={campaignId ?? ''}
              onChange={(e) => setCampaign(e.target.value)}
            >
              <option value="">All Campaigns</option>
              {campaigns.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </Select>
          </div>

          {/* Duration Filter */}
          <div className="flex items-center gap-2.5">
            <span className="text-xs font-black uppercase tracking-wider text-zinc-400 flex items-center gap-1">
              <Calendar className="h-3.5 w-3.5" />
              Period:
            </span>
            <Select
              className="w-32 rounded-2xl border border-zinc-200/50 bg-white/50 py-2 focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-xs font-bold"
              value={duration ?? ''}
              onChange={(e) => setDuration(e.target.value)}
            >
              <option value="">All Time</option>
              <option value="1d">Today</option>
              <option value="7d">1 Week</option>
              <option value="15d">15 Days</option>
              <option value="30d">1 Month</option>
              <option value="custom">Custom Date</option>
            </Select>
          </div>

          {/* Source Filter */}
          <div className="flex items-center gap-2.5">
            <span className="text-xs font-black uppercase tracking-wider text-zinc-400">Source:</span>
            <Select
              className="w-32 rounded-2xl border border-zinc-200/50 bg-white/50 py-2 focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-xs font-bold"
              value={source ?? ''}
              onChange={(e) => setSource(e.target.value)}
            >
              <option value="">All Sources</option>
              {sources.map((src) => (
                <option key={src} value={src}>
                  {src}
                </option>
              ))}
            </Select>
          </div>

          {/* Status Filter */}
          <div className="flex items-center gap-2.5">
            <span className="text-xs font-black uppercase tracking-wider text-zinc-400">Status:</span>
            <Select
              className="w-32 rounded-2xl border border-zinc-200/50 bg-white/50 py-2 focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-xs font-bold"
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
      </div>

      {/* Custom Date Picker Inputs */}
      {duration === 'custom' && (
        <div className="flex flex-wrap items-center gap-4 pt-3 border-t border-zinc-200/20 dark:border-zinc-800/25 animate-in fade-in slide-in-from-top-2 duration-300">
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold text-zinc-400">From:</span>
            <input
              type="date"
              value={localStartDate}
              onChange={(e) => {
                setLocalStartDate(e.target.value);
                handleCustomDateChange(e.target.value, localEndDate);
              }}
              className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-800 dark:text-white"
            />
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold text-zinc-400">To:</span>
            <input
              type="date"
              value={localEndDate}
              onChange={(e) => {
                setLocalEndDate(e.target.value);
                handleCustomDateChange(localStartDate, e.target.value);
              }}
              className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-800 dark:text-white"
            />
          </div>
        </div>
      )}
    </div>
  );
}

