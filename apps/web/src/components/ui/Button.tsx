import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'outline' | 'danger';
export type ButtonSize = 'sm' | 'md' | 'lg';

const variantClasses: Record<ButtonVariant, string> = {
  primary: cn(
    'bg-gradient-to-br from-brand-500 to-brand-700 text-white shadow-lg shadow-brand-500/20',
    'hover:shadow-xl hover:shadow-brand-500/30 hover:-translate-y-0.5 hover:scale-[1.02]',
    'active:translate-y-0 active:scale-[0.98]',
  ),
  secondary: cn(
    'bg-white/60 text-zinc-800 dark:bg-white/10 dark:text-zinc-100 shadow-sm backdrop-blur-md border border-white/20 dark:border-white/5',
    'hover:bg-white/80 dark:hover:bg-white/20 hover:-translate-y-0.5 hover:scale-[1.02]',
    'active:translate-y-0 active:scale-[0.98]',
  ),
  ghost: cn(
    'bg-transparent text-zinc-600 dark:text-zinc-400',
    'hover:bg-zinc-100/50 dark:hover:bg-white/5 hover:text-zinc-900 dark:hover:text-white',
    'active:scale-[0.98]',
  ),
  outline: cn(
    'bg-transparent text-zinc-700 dark:text-zinc-200',
    'border-2 border-zinc-200 dark:border-white/10',
    'hover:bg-white/50 dark:hover:bg-white/5 hover:border-brand-500 hover:-translate-y-0.5',
    'active:translate-y-0 active:scale-[0.98]',
  ),
  danger: cn(
    'bg-red-500 text-white shadow-lg shadow-red-500/20',
    'hover:bg-red-600 hover:shadow-xl hover:shadow-red-500/30 hover:-translate-y-0.5',
    'active:translate-y-0 active:scale-[0.98]',
  ),
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'h-9 px-4 text-xs gap-1.5 rounded-2xl',
  md: 'h-11 px-6 text-sm gap-2 rounded-[20px]',
  lg: 'h-14 px-8 text-base gap-2.5 rounded-[24px]',
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
