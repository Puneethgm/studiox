import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'outline' | 'danger';
export type ButtonSize = 'sm' | 'md' | 'lg';

const variantClasses: Record<ButtonVariant, string> = {
  primary: cn(
    'bg-brand-500 text-white',
    'hover:bg-brand-600',
    'active:scale-[0.98]',
  ),
  secondary: cn(
    'bg-white text-zinc-800 dark:bg-zinc-900 dark:text-zinc-100 border border-zinc-200 dark:border-zinc-800',
    'hover:bg-zinc-50 dark:hover:bg-zinc-800/50',
    'active:scale-[0.98]',
  ),
  ghost: cn(
    'bg-transparent text-zinc-600 dark:text-zinc-400',
    'hover:bg-zinc-100/50 dark:hover:bg-white/5 hover:text-zinc-900 dark:hover:text-white',
    'active:scale-[0.98]',
  ),
  outline: cn(
    'bg-transparent text-zinc-700 dark:text-zinc-200',
    'border border-zinc-200 dark:border-zinc-800',
    'hover:bg-zinc-50 dark:hover:bg-zinc-900/50 hover:border-zinc-300 dark:hover:border-zinc-700',
    'active:scale-[0.98]',
  ),
  danger: cn(
    'bg-red-500 text-white',
    'hover:bg-red-600',
    'active:scale-[0.98]',
  ),
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'h-9 px-4 text-xs gap-1.5 rounded',
  md: 'h-10 px-6 text-sm gap-2 rounded',
  lg: 'h-12 px-8 text-base gap-2.5 rounded',
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = 'primary',
    size = 'md',
    loading = false,
    leftIcon,
    rightIcon,
    children,
    className,
    type = 'button',
    disabled,
    ...rest
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      className={cn(
        'inline-flex items-center justify-center font-medium whitespace-nowrap select-none',
        'transition-all duration-150',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--brand,#7c3aed)] focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-slate-950',
        'disabled:pointer-events-none disabled:opacity-50',
        variantClasses[variant],
        sizeClasses[size],
        className,
      )}
      suppressHydrationWarning
      {...rest}
    >
      {loading ? <Spinner className="h-4 w-4" /> : leftIcon ? <span className="flex shrink-0 items-center">{leftIcon}</span> : null}
      {children && <span>{children}</span>}
      {!loading && rightIcon && <span className="flex shrink-0 items-center">{rightIcon}</span>}
    </button>
  );
});

function Spinner({ className }: { className?: string }) {
  return (
    <svg className={cn('animate-spin', className)} viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" className="opacity-25" />
      <path d="M22 12a10 10 0 0 1-10 10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" className="opacity-75" />
    </svg>
  );
}
