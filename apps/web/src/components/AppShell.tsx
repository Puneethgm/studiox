'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
  Building2,
  Home,
  Inbox,
  KanbanSquare,
  Lock,
  LogOut,
  Megaphone,
  Menu,
  MessagesSquare,
  Plug,
  Settings,
  Sparkles,
  X,
  CreditCard,
  Database,
  ChevronLeft,
  ChevronRight,
  HelpCircle,
  Sun,
  Moon,
} from 'lucide-react';
import { useEffect, useState, type CSSProperties, type ReactNode } from 'react';
import { Button } from '@/components/ui/Button';
import { api } from '@/lib/api';
import { brandInitials, withAlpha } from '@/lib/color';
import { cn } from '@/lib/cn';
import type { Me } from '@/lib/types';

interface NavItem {
  href: string;
  label: string;
  icon: ReactNode;
  match?: (pathname: string) => boolean;
}

function navItemsFor(me: Me): NavItem[] {
  if (me.role === 'super_admin') {
    return [
      {
        href: '/admin/studios',
        label: 'Studios',
        icon: <Building2 className="h-[18px] w-[18px]" />,
        match: (p) => p === '/admin' || p === '/admin/studios' || (p.startsWith('/admin/studios/') && !p.includes('/inbox') && !p.includes('/pipeline') && !p.includes('/campaigns') && !p.includes('/leads') && !p.includes('/channels') && !p.includes('/settings')),
      },
      {
        href: '/admin/settings',
        label: 'Settings',
        icon: <Settings className="h-[18px] w-[18px]" />,
        match: (p) => p.startsWith('/admin/settings'),
      },
    ];
  }
  const sid = me.studioId!;
  const base = `/admin/studios/${sid}`;
  const links: NavItem[] = [
    { href: base,                 label: 'Dashboard', icon: <Home className="h-[18px] w-[18px]" />,           match: (p) => p === base },
    { href: `${base}/inbox`,      label: 'Inbox',     icon: <MessagesSquare className="h-[18px] w-[18px]" />, match: (p) => p.startsWith(`${base}/inbox`) },
    { href: `${base}/pipeline`,   label: 'Pipeline',  icon: <KanbanSquare className="h-[18px] w-[18px]" />,   match: (p) => p.startsWith(`${base}/pipeline`) },
    { href: `${base}/campaigns`,  label: 'Campaigns', icon: <Megaphone className="h-[18px] w-[18px]" />,      match: (p) => p.startsWith(`${base}/campaigns`) },
    { href: `${base}/leads`,      label: 'Leads',     icon: <Inbox className="h-[18px] w-[18px]" />,          match: (p) => p.startsWith(`${base}/leads`) },
  ];
  
  if (me.studio?.socialPlannerEnabled) {
    links.push({ href: `${base}/social-planner`, label: 'Social Planner', icon: <Sparkles className="h-[18px] w-[18px]" />, match: (p) => p.startsWith(`${base}/social-planner`) });
  }
  
  links.push(
    { href: `${base}/payments`,   label: 'Payments',  icon: <CreditCard className="h-[18px] w-[18px]" />,     match: (p) => p.startsWith(`${base}/payments`) },
    { href: `${base}/channels`,   label: 'Channels',  icon: <Plug className="h-[18px] w-[18px]" />,           match: (p) => p.startsWith(`${base}/channels`) },
    { href: `${base}/knowledge-base`, label: 'Knowledge Base', icon: <Database className="h-[18px] w-[18px]" />, match: (p) => p.startsWith(`${base}/knowledge-base`) },
    { href: `${base}/settings`,   label: 'Settings',  icon: <Settings className="h-[18px] w-[18px]" />,       match: (p) => p.startsWith(`${base}/settings`) }
  );
  
  return links;
}

export function AppShell({ me, children }: { me: Me; children: ReactNode }) {
  const isStudio = me.role === 'studio_admin' && !!me.studio;
  const brand = isStudio ? me.studio!.brandColor : '#7c3aed';

  // All hooks must run unconditionally — keep them above the lockout branch.
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isCollapsed, setIsCollapsed] = useState(false);

  useEffect(() => {
    const val = localStorage.getItem('sidebar-collapsed');
    if (val !== null) {
      setIsCollapsed(val === 'true');
    }
  }, []);

  const handleToggleSidebar = () => {
    setIsCollapsed(prev => {
      const next = !prev;
      localStorage.setItem('sidebar-collapsed', String(next));
      return next;
    });
  };

  // Auto-close the drawer on navigation.
  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  // Lock body scroll while the mobile drawer is open.
  useEffect(() => {
    if (mobileOpen) {
      document.body.style.overflow = 'hidden';
      return () => {
        document.body.style.overflow = '';
      };
    }
  }, [mobileOpen]);

  const themeStyle: CSSProperties = {
    ['--brand' as string]: brand,
    ['--brand-soft' as string]: withAlpha(brand, 0.08),
    ['--brand-softer' as string]: withAlpha(brand, 0.16),
    ['--brand-onbrand' as string]: '#ffffff',
  };

  // Studio-admin of an inactive studio: full-screen lockout. The backend
  // also 403s every studio-scoped endpoint with `studio_inactive`; this is
  // the matching UX wrapper. Super-admin always sees the normal shell.
  if (isStudio && me.studio!.active === false) {
    return <StudioInactiveScreen me={me} />;
  }

  return (
    <div
      className="min-h-screen text-zinc-900 dark:text-zinc-100"
      style={themeStyle}
    >
      {/* Mobile backdrop */}
      <div
        className={cn(
          'fixed inset-0 z-40 bg-neutral-950/40 backdrop-blur-md transition-opacity lg:hidden',
          mobileOpen ? 'opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={() => setMobileOpen(false)}
        aria-hidden
      />

      <div className="lg:flex lg:h-screen lg:gap-4 lg:p-4">
        <Sidebar 
          me={me} 
          mobileOpen={mobileOpen} 
          onClose={() => setMobileOpen(false)} 
          isCollapsed={isCollapsed}
          onToggle={handleToggleSidebar}
        />
        <div className="flex min-w-0 flex-1 flex-col overflow-hidden lg:glass-container">
          <Topbar me={me} onMenuClick={() => setMobileOpen(true)} />
          <main className="relative flex-1 overflow-y-auto px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
            {children}
          </main>
        </div>
      </div>
    </div>
  );
}

function Sidebar({
  me,
  mobileOpen,
  onClose,
  isCollapsed,
  onToggle,
}: {
  me: Me;
  mobileOpen: boolean;
  onClose: () => void;
  isCollapsed: boolean;
  onToggle: () => void;
}) {
  const pathname = usePathname();
  const items = navItemsFor(me);
  const isStudio = me.role === 'studio_admin' && !!me.studio;
  const studio = isStudio ? me.studio! : null;

  const [logoError, setLogoError] = useState(false);
  useEffect(() => {
    setLogoError(false);
  }, [studio?.logoUrl]);

  return (
    <aside
      className={cn(
        // Mobile: fixed drawer that slides in from the left
        'fixed inset-y-4 left-4 z-50 flex w-72 flex-col overflow-hidden rounded-[32px] border border-white/40 bg-white/40 px-4 py-6 backdrop-blur-3xl transition-all duration-500 ease-[cubic-bezier(0.34,1.56,0.64,1)]',
        'dark:border-white/10 dark:bg-neutral-900/40',
        // Desktop: sticky in flow, always visible
        'lg:relative lg:inset-y-0 lg:left-0 lg:z-auto lg:h-full lg:translate-x-0 lg:rounded-[40px] lg:shadow-liquid lg:transition-[width,padding] lg:duration-300 lg:ease-out',
        isCollapsed 
          ? 'lg:w-20 lg:items-center lg:px-2' 
          : 'lg:w-64 lg:items-stretch lg:px-4',
        mobileOpen ? 'translate-x-0 shadow-2xl' : '-translate-x-[calc(100%+20px)]',
      )}
      aria-label="Primary navigation"
    >
      {/* Mobile-only close button */}
      <button
        type="button"
        onClick={onClose}
        className="absolute right-3 top-3 grid h-8 w-8 place-items-center rounded-lg text-slate-500 hover:bg-slate-100 hover:text-slate-900 lg:hidden dark:hover:bg-slate-800 dark:hover:text-slate-100"
        aria-label="Close menu"
        suppressHydrationWarning
      >
        <X className="h-4 w-4" />
      </button>

      {/* Brand block */}
      {isStudio ? (
        <div className={cn(
          "mb-10 flex animate-in items-center gap-3 lg:transition-all lg:duration-300",
          isCollapsed ? "lg:flex-col lg:gap-1" : "lg:flex-row lg:gap-3"
        )} style={{ animationDelay: '100ms' }}>
          <div
            className="grid h-12 w-12 shrink-0 animate-float place-items-center overflow-hidden rounded-2xl text-sm font-bold text-white shadow-lg shadow-brand-500/20 ring-4 ring-white/30"
            style={{ background: studio!.brandColor }}
          >
            {studio!.logoUrl && !logoError ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={studio!.logoUrl} alt="" className="h-12 w-12 object-cover" onError={() => setLogoError(true)} />
            ) : (
              brandInitials(studio!.name)
            )}
          </div>
          <div className={cn(
            "min-w-0 lg:block lg:transition-all lg:duration-300",
            isCollapsed ? "lg:max-w-0 lg:overflow-hidden lg:opacity-0" : "lg:max-w-[11rem] lg:opacity-100"
          )}>
            <div className="truncate text-sm font-bold text-zinc-900 dark:text-zinc-100">
              {studio!.name}
            </div>
            <div className="truncate font-mono text-[10px] font-medium text-zinc-500">
              /{studio!.slug}
            </div>
          </div>
        </div>
      ) : (
        <div className={cn(
          "mb-10 flex animate-in items-center gap-3 lg:transition-all lg:duration-300",
          isCollapsed ? "lg:flex-col lg:gap-1" : "lg:flex-row lg:gap-3"
        )} style={{ animationDelay: '100ms' }}>
          <div className="grid h-12 w-12 shrink-0 animate-float place-items-center rounded-2xl bg-gradient-to-br from-brand-300 via-brand-primary to-brand-700 text-sm font-extrabold text-white shadow-lg shadow-brand-500/20 ring-4 ring-white/30">
            1H
          </div>
          <div className={cn(
            "min-w-0 lg:block lg:transition-all lg:duration-300",
            isCollapsed ? "lg:max-w-0 lg:overflow-hidden lg:opacity-0" : "lg:max-w-[11rem] lg:opacity-100"
          )}>
            <div className="truncate text-sm font-bold text-zinc-900 dark:text-zinc-100">
              1herosocial.ai
            </div>
            <div className="text-[10px] font-medium text-zinc-500">Platform admin</div>
          </div>
        </div>
      )}

      {/* Nav */}
      <nav className={cn(
        "flex flex-1 flex-col gap-3 overflow-y-auto no-scrollbar lg:w-full",
        isCollapsed ? "lg:items-center" : "lg:items-stretch"
      )}>
        {items.map((item, idx) => {
          const active = item.match ? item.match(pathname) : pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'group flex animate-in items-center gap-3 rounded-[20px] px-3 py-3 text-sm font-semibold transition-colors duration-300 lg:transition-[background-color,color,box-shadow,padding,transform] lg:duration-300',
                active
                  ? 'text-white shadow-lg'
                  : 'text-zinc-500 hover:bg-white/50 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-white/5 dark:hover:text-zinc-100',
                isCollapsed ? 'lg:justify-center' : 'lg:justify-start',
              )}
              style={{
                animationDelay: `${150 + idx * 50}ms`,
                ...(active ? { background: 'linear-gradient(135deg, var(--brand) 0%, #a78bfa 100%)', boxShadow: '0 8px 16px -4px rgba(124, 58, 237, 0.3)' } : {}),
              }}
            >
              <span className={cn('shrink-0 transition-transform duration-300 group-hover:scale-110', active && '[&>svg]:stroke-[2.5]')}>
                {item.icon}
              </span>
              <span className={cn(
                "truncate lg:block lg:transition-all lg:duration-300",
                isCollapsed ? "lg:max-w-0 lg:overflow-hidden lg:opacity-0" : "lg:max-w-[12rem] lg:opacity-100"
              )}>
                {item.label}
              </span>
            </Link>
          );
        })}
      </nav>

      {/* Footer hint */}
      <div className={cn(
        "mt-6 mb-4 animate-in rounded-3xl border border-white/20 bg-white/20 text-xs text-zinc-500 backdrop-blur-md dark:border-white/5 dark:bg-black/20 lg:block lg:transition-all lg:duration-300",
        isCollapsed ? "lg:max-h-0 lg:overflow-hidden lg:p-0 lg:opacity-0" : "lg:max-h-40 lg:p-4 lg:opacity-100"
      )} style={{ animationDelay: '500ms' }}>
        <div className="flex items-center gap-2 font-black text-zinc-800 dark:text-zinc-200">
          <Sparkles className="h-3.5 w-3.5 text-brand-500" />
          Tip
        </div>
        <p className="mt-1.5 leading-relaxed font-medium">
          {isStudio
            ? 'Drop your campaign link in your Instagram bio to start collecting leads.'
            : 'Studios sign in at the same /login URL — their account routes them to their own dashboard.'}
        </p>
      </div>

      {/* Sidebar Footer Controls */}
      <div className="mt-auto flex items-center justify-center w-full pt-4 border-t border-white/20 dark:border-white/5 shrink-0">
        <button
          type="button"
          onClick={onToggle}
          className="flex h-9 w-9 items-center justify-center rounded-xl bg-white/20 hover:bg-white/50 border border-white/20 dark:bg-black/20 dark:hover:bg-white/5 dark:border-white/5 text-zinc-500 hover:text-zinc-800 dark:hover:text-white transition-all duration-300"
          title={isCollapsed ? "Expand" : "Collapse"}
        >
          {isCollapsed ? (
            <ChevronRight className="h-5 w-5" />
          ) : (
            <ChevronLeft className="h-5 w-5" />
          )}
        </button>
      </div>
    </aside>
  );
}

function ThemeToggle() {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');

  useEffect(() => {
    const isDark = document.documentElement.classList.contains('dark');
    setTheme(isDark ? 'dark' : 'light');
  }, []);

  const toggleTheme = () => {
    const nextTheme = theme === 'light' ? 'dark' : 'light';
    setTheme(nextTheme);
    if (nextTheme === 'dark') {
      document.documentElement.classList.add('dark');
      localStorage.setItem('theme', 'dark');
    } else {
      document.documentElement.classList.remove('dark');
      localStorage.setItem('theme', 'light');
    }
  };

  return (
    <button
      onClick={toggleTheme}
      className="grid h-10 w-10 place-items-center rounded-xl border border-white/20 bg-white/20 dark:bg-black/20 dark:border-white/5 text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-white transition-all duration-300 shadow-sm shrink-0"
      aria-label="Toggle theme"
      title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
    >
      {theme === 'light' ? (
        <Moon className="h-[18px] w-[18px]" />
      ) : (
        <Sun className="h-[18px] w-[18px]" />
      )}
    </button>
  );
}

function Topbar({ me, onMenuClick }: { me: Me; onMenuClick: () => void }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);

  async function logout() {
    try {
      await api('/api/v1/auth/logout', { method: 'POST' });
    } finally {
      router.push('/login');
      router.refresh();
    }
  }

  return (
    <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-3 border-b border-white/20 bg-white/60 px-4 backdrop-blur-2xl sm:px-6 lg:justify-end lg:px-10 dark:border-white/5 dark:bg-neutral-950/60" style={{ boxShadow: '0 1px 0 rgba(255,255,255,0.1), 0 4px 20px rgba(0,0,0,0.04)' }}>
      {/* Mobile menu button */}
      <button
        type="button"
        onClick={onMenuClick}
        className="grid h-10 w-10 place-items-center rounded-xl text-slate-700 hover:bg-slate-100 lg:hidden dark:text-slate-200 dark:hover:bg-slate-800"
        aria-label="Open menu"
        suppressHydrationWarning
      >
        <Menu className="h-5 w-5" />
      </button>

      <div className="flex items-center gap-3">
        {/* Theme Toggle Button */}
        <ThemeToggle />

        <div className="relative">
          <button
            onClick={() => setOpen(!open)}
            className="flex items-center gap-3 rounded-2xl p-1 pr-3 transition-all duration-300 hover:bg-slate-100 dark:hover:bg-slate-800"
            suppressHydrationWarning
          >
            <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-brand-400 to-brand-600 text-sm font-bold text-white shadow-lg ring-2 ring-white dark:ring-slate-900">
              {(me.email[0] ?? '').toUpperCase()}
            </div>
            <div className="hidden flex-col items-start sm:flex">
              <span className="max-w-[150px] truncate text-sm font-bold text-slate-900 dark:text-slate-100">
                {me.email.split('@')[0]}
              </span>
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500">
                {me.role.replace('_', ' ')}
              </span>
            </div>
            <Menu className={cn("h-4 w-4 text-slate-400 transition-transform duration-300", open && "rotate-90")} />
          </button>

          {open && (
            <>
              <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
              <div className="absolute right-0 top-full mt-3 w-64 animate-in overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-2xl backdrop-blur-xl dark:border-slate-800 dark:bg-slate-900/90 z-20">
                <div className="border-b border-slate-100 p-5 dark:border-slate-800/60">
                  <div className="flex items-center gap-3">
                    <div className="grid h-12 w-12 place-items-center rounded-2xl bg-brand-500 text-lg font-bold text-white">
                      {(me.email[0] ?? '').toUpperCase()}
                    </div>
                    <div className="min-w-0">
                      <div className="truncate text-base font-bold text-slate-900 dark:text-slate-100">{me.email.split('@')[0]}</div>
                      <div className="truncate text-xs text-slate-500">{me.email}</div>
                    </div>
                  </div>
                </div>
                <div className="p-2">
                  <button
                    onClick={logout}
                    className="flex w-full items-center gap-3 rounded-xl px-4 py-3 text-sm font-bold text-red-600 transition-colors hover:bg-red-50 dark:hover:bg-red-500/10"
                    suppressHydrationWarning
                  >
                    <LogOut className="h-4 w-4" />
                    Sign out
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </header>
  );
}

// Full-screen lockout for studio-admins whose studio has been deactivated.
// Mirrors the backend 403 — they can sign out, but cannot navigate anywhere
// in the admin. Super-admins never see this (their AppShell branch skips it).
function StudioInactiveScreen({ me }: { me: Me }) {
  const router = useRouter();
  const studio = me.studio!;

  async function logout() {
    try {
      await api('/api/v1/auth/logout', { method: 'POST' });
    } finally {
      router.push('/login');
      router.refresh();
    }
  }

  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 px-4 dark:bg-slate-950">
      <div className="w-full max-w-md text-center">
        <div className="mx-auto mb-5 grid h-14 w-14 place-items-center rounded-2xl bg-slate-900/5 text-slate-500 dark:bg-slate-50/5 dark:text-slate-400">
          <Lock className="h-6 w-6" />
        </div>
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
          {studio.name} is inactive
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-slate-500 dark:text-slate-400">
          The platform admin has paused this studio. You can&rsquo;t access campaigns,
          leads, or settings until it&rsquo;s reactivated. Reach out to your platform
          admin if you think this is a mistake.
        </p>
        <div className="mt-8">
          <Button variant="outline" onClick={logout} leftIcon={<LogOut className="h-4 w-4" />} suppressHydrationWarning>
            Sign out
          </Button>
        </div>
      </div>
    </main>
  );
}
