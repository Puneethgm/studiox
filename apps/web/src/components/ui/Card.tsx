import type { HTMLAttributes, ReactNode } from 'react';
import { cn } from '@/lib/cn';

// Intentionally NOT extending HTMLAttributes — DOM `title` is `string`, ours is ReactNode.
export interface CardProps {
  title?: ReactNode;
  subtitle?: ReactNode;
  action?: ReactNode;
  footer?: ReactNode;
  elevated?: boolean;
  glass?: boolean;
  noPadding?: boolean;
  className?: string;
  children?: ReactNode;
  id?: HTMLAttributes<HTMLDivElement>['id'];
}

export function Card({
  title, subtitle, action, footer, elevated = false, glass = false, noPadding = false,
  className, children, id,
}: CardProps) {
  return (
    <div
      id={id}
      className={cn(
        'rounded-[32px] transition-all duration-500',
        glass ? 'glass' : 'bg-white/70 backdrop-blur-xl dark:bg-neutral-900/70',
        'border border-white/20 dark:border-white/5',
        elevated ? 'shadow-liquid scale-[1.02]' : 'shadow-sm',
        className,
      )}
    >
      {(title || action) && (
        <div className="flex items-start justify-between gap-4 border-b border-white/10 px-6 py-5 dark:border-white/5">
          <div className="min-w-0">
            {title && (
              <h3 className="text-lg font-bold leading-tight text-zinc-900 dark:text-zinc-100">
                {title}
              </h3>
            )}
            {subtitle && <p className="mt-1 text-sm font-medium text-zinc-500 dark:text-zinc-400">{subtitle}</p>}
          </div>
          {action && <div className="shrink-0">{action}</div>}
        </div>
      )}
      <div className={cn(noPadding ? '' : 'p-6')}>{children}</div>
      {footer && (
        <div className="border-t border-white/10 bg-white/20 px-6 py-4 dark:border-white/5 dark:bg-black/20">
          {footer}
        </div>
      )}
    </div>
  );
}
