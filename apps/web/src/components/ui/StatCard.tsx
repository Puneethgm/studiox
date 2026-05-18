import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';

export type StatCardColor = 'sky' | 'violet' | 'emerald' | 'amber' | 'rose' | 'default';

export interface StatCardProps {
  label: string;
  value: ReactNode;
  icon?: ReactNode;
  href?: string;
  hint?: ReactNode;
  color?: StatCardColor;
  className?: string;
}

const colorConfig: Record<StatCardColor, {
  bg: string;
  border: string;
  iconBg: string;
  iconText: string;
  glow: string;
  valueText: string;
  hoverBg: string;
  shadow: string;
}> = {
  sky: {
    bg: 'bg-sky-50/40 dark:bg-sky-950/20',
    border: 'border-sky-200/50 dark:border-sky-500/15',
    iconBg: 'bg-sky-100/70 dark:bg-sky-500/15',
    iconText: 'text-sky-600 dark:text-sky-400',
    glow: 'from-sky-400/20 to-blue-400/10',
    valueText: 'text-sky-900 dark:text-sky-100',
    hoverBg: 'hover:bg-sky-50/60 dark:hover:bg-sky-950/30',
    shadow: '0 4px 20px rgba(14,165,233,0.10), inset 0 0 0 1px rgba(186,230,253,0.4)',
  },
  violet: {
    bg: 'bg-violet-50/40 dark:bg-violet-950/20',
    border: 'border-violet-200/50 dark:border-violet-500/15',
    iconBg: 'bg-violet-100/70 dark:bg-violet-500/15',
    iconText: 'text-violet-600 dark:text-violet-400',
    glow: 'from-violet-400/20 to-purple-400/10',
    valueText: 'text-violet-900 dark:text-violet-100',
    hoverBg: 'hover:bg-violet-50/60 dark:hover:bg-violet-950/30',
    shadow: '0 4px 20px rgba(139,92,246,0.10), inset 0 0 0 1px rgba(221,214,254,0.4)',
  },
  emerald: {
    bg: 'bg-emerald-50/40 dark:bg-emerald-950/20',
    border: 'border-emerald-200/50 dark:border-emerald-500/15',
    iconBg: 'bg-emerald-100/70 dark:bg-emerald-500/15',
    iconText: 'text-emerald-600 dark:text-emerald-400',
    glow: 'from-emerald-400/20 to-teal-400/10',
    valueText: 'text-emerald-900 dark:text-emerald-100',
    hoverBg: 'hover:bg-emerald-50/60 dark:hover:bg-emerald-950/30',
    shadow: '0 4px 20px rgba(16,185,129,0.10), inset 0 0 0 1px rgba(167,243,208,0.4)',
  },
  amber: {
    bg: 'bg-amber-50/40 dark:bg-amber-950/20',
    border: 'border-amber-200/50 dark:border-amber-500/15',
    iconBg: 'bg-amber-100/70 dark:bg-amber-500/15',
    iconText: 'text-amber-600 dark:text-amber-400',
    glow: 'from-amber-400/20 to-orange-400/10',
    valueText: 'text-amber-900 dark:text-amber-100',
    hoverBg: 'hover:bg-amber-50/60 dark:hover:bg-amber-950/30',
    shadow: '0 4px 20px rgba(245,158,11,0.10), inset 0 0 0 1px rgba(253,230,138,0.4)',
  },
  rose: {
    bg: 'bg-rose-50/40 dark:bg-rose-950/20',
    border: 'border-rose-200/50 dark:border-rose-500/15',
    iconBg: 'bg-rose-100/70 dark:bg-rose-500/15',
    iconText: 'text-rose-600 dark:text-rose-400',
    glow: 'from-rose-400/20 to-pink-400/10',
    valueText: 'text-rose-900 dark:text-rose-100',
    hoverBg: 'hover:bg-rose-50/60 dark:hover:bg-rose-950/30',
    shadow: '0 4px 20px rgba(244,63,94,0.10), inset 0 0 0 1px rgba(254,205,211,0.4)',
  },
  default: {
    bg: 'bg-white/30 dark:bg-neutral-900/30',
    border: 'border-white/30 dark:border-white/5',
    iconBg: 'bg-white/50 dark:bg-white/10',
    iconText: 'text-zinc-600 dark:text-zinc-300',
    glow: 'from-white/20 to-white/10',
    valueText: 'text-zinc-900 dark:text-white',
    hoverBg: 'hover:bg-white/40 dark:hover:bg-neutral-800/40',
    shadow: '0 4px 16px rgba(0,0,0,0.05), inset 0 0 0 1px rgba(255,255,255,0.15)',
  },
};

export function StatCard({ label, value, icon, href, hint, color = 'default', className }: StatCardProps) {
  const c = colorConfig[color];

  const inner = (
    <div
      className={cn(
        'group relative overflow-hidden rounded-[22px] border p-5 backdrop-blur-2xl transition-all duration-500',
        c.bg, c.border,
        href && `${c.hoverBg} hover:-translate-y-1 hover:scale-[1.015]`,
        className,
      )}
      style={{ boxShadow: c.shadow }}
    >
      {/* Subtle glow blob top-right */}
      <div className={cn('pointer-events-none absolute -right-6 -top-6 h-24 w-24 rounded-full bg-gradient-to-br blur-2xl opacity-60', c.glow)} />

      <div className="relative flex items-start justify-between gap-3">
        {/* Icon */}
        {icon && (
          <span className={cn(
            'mt-0.5 grid h-10 w-10 shrink-0 place-items-center rounded-2xl backdrop-blur-sm transition-transform duration-500 group-hover:scale-110 group-hover:rotate-6',
            c.iconBg, c.iconText,
          )}>
            {icon}
          </span>
        )}

        {/* Text */}
        <div className="min-w-0 flex-1">
          <div className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400 dark:text-zinc-500">
            {label}
          </div>
          <div className={cn('mt-1.5 text-3xl font-black tracking-tight', c.valueText)}>
            {value}
          </div>
          {hint && (
            <div className="mt-1 text-[11px] font-semibold text-zinc-500 dark:text-zinc-400">{hint}</div>
          )}
        </div>

        {/* Arrow on hover */}
        {href && (
          <div className={cn(
            'mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-xl opacity-0 transition-all duration-500 group-hover:opacity-100',
            c.iconBg, c.iconText,
          )}>
            <ArrowUpRight className="h-4 w-4" />
          </div>
        )}
      </div>
    </div>
  );

  return href
    ? <Link href={href} className={cn('block outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 rounded-[22px]')}>{inner}</Link>
    : inner;
}
