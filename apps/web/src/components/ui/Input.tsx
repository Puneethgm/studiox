import { forwardRef, type InputHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

const fieldBase = cn(
  'w-full rounded-xl border bg-white text-slate-900 shadow-sm transition-all duration-200',
  'border-slate-200 placeholder:text-slate-400',
  'focus-visible:outline-none focus-visible:border-[color:var(--brand,#7c3aed)] focus-visible:ring-4 focus-visible:ring-[color:var(--brand-soft,rgba(124,58,237,0.08))]',
  'hover:border-slate-300 dark:hover:border-slate-700',
  'dark:bg-slate-950 dark:text-slate-100 dark:border-slate-800 dark:placeholder:text-slate-600',
  'disabled:cursor-not-allowed disabled:opacity-60',
);

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, invalid, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(fieldBase, 'h-10 px-3 text-sm', invalid && 'border-red-500 focus-visible:ring-red-500/30', className)}
      suppressHydrationWarning
      {...rest}
    />
  );
});
