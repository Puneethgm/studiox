import { forwardRef, type TextareaHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

const fieldBase = cn(
  'w-full rounded-lg border bg-white text-slate-900 shadow-sm transition-colors',
  'border-slate-300 placeholder:text-slate-400',
  'focus-visible:outline-none focus-visible:border-[color:var(--brand,#7c3aed)] focus-visible:ring-2 focus-visible:ring-[color:var(--brand-softer,rgba(124,58,237,0.18))]',
  'dark:bg-slate-900 dark:text-slate-100 dark:border-slate-700 dark:placeholder:text-slate-500',
    'disabled:cursor-not-allowed disabled:opacity-60',
);

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, invalid, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={cn(fieldBase, 'min-h-24 px-3 py-2 text-sm', invalid && 'border-red-500 focus-visible:ring-red-500/30', className)}
      suppressHydrationWarning
      {...rest}
    />
  );
});
