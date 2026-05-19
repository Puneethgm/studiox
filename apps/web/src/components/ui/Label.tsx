import { forwardRef, type LabelHTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export const Label = forwardRef<HTMLLabelElement, LabelHTMLAttributes<HTMLLabelElement>>(
  function Label({ className, ...rest }, ref) {
    return (
      <label
        ref={ref}
        className={cn('mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200', className)}
        {...rest}
      />
    );
  },
);

export function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="mt-1.5 text-xs font-medium text-red-600 dark:text-red-400">{message}</p>;
}

export function FieldHint({ children, ...props }: { children: ReactNode } & React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className="mt-1.5 text-xs text-slate-500 dark:text-slate-400" {...props}>{children}</p>;
}
