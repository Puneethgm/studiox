import { forwardRef, type SelectHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

const fieldBase = cn(
  'w-full rounded-lg border bg-white text-slate-900 shadow-sm transition-colors',
  'border-slate-300',
  'focus-visible:outline-none focus-visible:border-[color:var(--brand,#7c3aed)] focus-visible:ring-2 focus-visible:ring-[color:var(--brand-softer,rgba(124,58,237,0.18))]',
  'dark:bg-slate-900 dark:text-slate-100 dark:border-slate-700',
    'disabled:cursor-not-allowed disabled:opacity-60',
);

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className, invalid, children, ...rest },
  ref,
) {
  return (
    <select
      ref={ref}
      className={cn(
        fieldBase,
        'h-10 pl-3 pr-9 text-sm appearance-none bg-no-repeat',
        'bg-[url("data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20viewBox=%220%200%2024%2024%22%20fill=%22none%22%20stroke=%22%2364748b%22%20stroke-width=%222%22%20stroke-linecap=%22round%22%20stroke-linejoin=%22round%22%3E%3Cpolyline%20points=%226%209%2012%2015%2018%209%22%3E%3C/polyline%3E%3C/svg%3E")] bg-[length:1rem_1rem] bg-[position:right_0.625rem_center]',
        invalid && 'border-red-500 focus-visible:ring-red-500/30',
        className,
      )}
      suppressHydrationWarning
      {...rest}
    >
      {children}
    </select>
  );
});
