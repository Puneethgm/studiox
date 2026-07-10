import { forwardRef, type InputHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

const fieldBase = cn(
  'w-full rounded border bg-white text-zinc-900 transition-all duration-200',
  'border-zinc-200 placeholder:text-zinc-400',
  'focus-visible:outline-none focus-visible:border-[color:var(--brand,#7c3aed)] focus-visible:ring-1 focus-visible:ring-[color:var(--brand,#7c3aed)]',
  'hover:border-zinc-300 dark:hover:border-zinc-700',
  'dark:bg-zinc-950 dark:text-zinc-100 dark:border-zinc-800 dark:placeholder:text-zinc-600',
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
