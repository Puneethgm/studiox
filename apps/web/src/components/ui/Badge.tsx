import type { HTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

export type BadgeTone = 'neutral' | 'brand' | 'success' | 'warning' | 'danger' | 'info';

const tones: Record<BadgeTone, string> = {
  neutral: 'bg-white/40 text-zinc-600 dark:bg-white/5 dark:text-zinc-400 border border-white/20 dark:border-white/5 backdrop-blur-md',
  brand:   'bg-brand-500/10 text-brand-700 dark:text-brand-300 border border-brand-500/20 backdrop-blur-md',
  success: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border border-emerald-500/20 backdrop-blur-md',
  warning: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border border-amber-500/20 backdrop-blur-md',
  danger:  'bg-red-500/10 text-red-700 dark:text-red-400 border border-red-500/20 backdrop-blur-md',
  info:    'bg-sky-500/10 text-sky-700 dark:text-sky-400 border border-sky-500/20 backdrop-blur-md',
};

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
}

export function Badge({ tone = 'neutral', className, ...rest }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        tones[tone],
        className,
      )}
      {...rest}
    />
  );
}
