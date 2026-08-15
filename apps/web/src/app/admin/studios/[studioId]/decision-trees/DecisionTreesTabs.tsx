'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/cn';

export function DecisionTreesTabs({ studioId }: { studioId: string }) {
  const pathname = usePathname();
  const isFollowUps = pathname?.endsWith('/decision-trees/follow-ups');

  const tabs = [
    { href: `/admin/studios/${studioId}/decision-trees`, label: 'Trees', active: !isFollowUps },
    { href: `/admin/studios/${studioId}/decision-trees/follow-ups`, label: 'Follow-ups', active: isFollowUps },
  ];

  return (
    <div className="flex gap-6 border-b border-zinc-200 dark:border-zinc-800">
      {tabs.map((t) => (
        <Link
          key={t.href}
          href={t.href}
          className={cn(
            'py-3 text-xs font-black uppercase tracking-wider transition-all border-b-2 whitespace-nowrap',
            t.active
              ? 'border-brand-500 text-brand-500 dark:text-brand-400'
              : 'border-transparent text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200'
          )}
        >
          {t.label}
        </Link>
      ))}
    </div>
  );
}
