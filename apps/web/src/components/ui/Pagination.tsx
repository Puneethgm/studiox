'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { cn } from '@/lib/cn';

export interface PaginationProps {
  total: number;
  pageSize: number;
  page: number;
  /** Optional: searchParam name. Defaults to "page". */
  paramName?: string;
}

export function Pagination({ total, pageSize, page, paramName = 'page' }: PaginationProps) {
  const searchParams = useSearchParams();
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(Math.max(1, page), totalPages);

  const start = total === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const end = Math.min(safePage * pageSize, total);

  function hrefFor(p: number): string {
    const params = new URLSearchParams(searchParams.toString());
    if (p > 1) params.set(paramName, String(p));
    else params.delete(paramName);
    const qs = params.toString();
    return qs ? `?${qs}` : '?';
  }

  if (totalPages <= 1) {
    return (
      <div className="mt-4 flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
        <span>
          {total === 0 ? 'No results' : `Showing ${start}–${end} of ${total}`}
        </span>
      </div>
    );
  }

  const pages = pageRange(safePage, totalPages);

  return (
    <div className="mt-5 flex flex-col-reverse items-stretch justify-between gap-3 sm:flex-row sm:items-center">
      <div className="text-xs text-slate-500 dark:text-slate-400">
        Showing <span className="font-medium text-slate-700 dark:text-slate-200">{start}–{end}</span> of{' '}
        <span className="font-medium text-slate-700 dark:text-slate-200">{total}</span>
      </div>

      <nav className="flex items-center gap-1" aria-label="Pagination">
        <PageLink href={safePage > 1 ? hrefFor(safePage - 1) : null} ariaLabel="Previous page">
          <ChevronLeft className="h-4 w-4" />
        </PageLink>

        {pages.map((p, i) =>
          p === 'ellipsis' ? (
            <span key={`ellipsis-${i}`} className="px-2 text-sm text-slate-400">
              …
            </span>
          ) : (
            <PageLink key={p} href={hrefFor(p)} active={p === safePage}>
              {p}
            </PageLink>
          ),
        )}

        <PageLink href={safePage < totalPages ? hrefFor(safePage + 1) : null} ariaLabel="Next page">
          <ChevronRight className="h-4 w-4" />
        </PageLink>
      </nav>
    </div>
  );
}

function PageLink({
  href,
  active,
  ariaLabel,
  children,
}: {
  href: string | null;
  active?: boolean;
  ariaLabel?: string;
  children: React.ReactNode;
}) {
  const className = cn(
    'inline-flex h-8 min-w-8 items-center justify-center rounded-md border px-2 text-sm font-medium transition-colors',
    active
      ? 'border-transparent text-white'
      : 'border-slate-200 bg-white text-slate-700 hover:border-[color:var(--brand,#7c3aed)] hover:text-[color:var(--brand,#7c3aed)] dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300',
    !href && 'pointer-events-none opacity-40',
  );
  const style = active ? { background: 'var(--brand, #7c3aed)' } : undefined;

  if (!href) {
    return (
      <span className={className} aria-disabled aria-label={ariaLabel}>
        {children}
      </span>
    );
  }
  return (
    <Link href={href} className={className} style={style} aria-current={active ? 'page' : undefined} aria-label={ariaLabel}>
      {children}
    </Link>
  );
}

// pageRange returns the page numbers to render, with 'ellipsis' tokens where
// there are gaps. Keeps a tight window around the current page.
function pageRange(current: number, total: number): (number | 'ellipsis')[] {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }
  const out: (number | 'ellipsis')[] = [1];
  const left = Math.max(2, current - 1);
  const right = Math.min(total - 1, current + 1);
  if (left > 2) out.push('ellipsis');
  for (let p = left; p <= right; p++) out.push(p);
  if (right < total - 1) out.push('ellipsis');
  out.push(total);
  return out;
}
