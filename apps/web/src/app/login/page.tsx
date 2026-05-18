'use client';

import { useRouter } from 'next/navigation';
import { useState, type CSSProperties } from 'react';
import { ArrowRight, Building2, Inbox, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';
import { withAlpha } from '@/lib/color';
import type { Me } from '@/lib/types';

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [postBrand, setPostBrand] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const me = await api<Me>('/api/v1/auth/login', {
        method: 'POST',
        json: { email, password },
      });
      if (me.role === 'studio_admin' && me.studioId) {
        setPostBrand(me.studio?.brandColor ?? null);
        router.push(`/admin/studios/${me.studioId}`);
      } else {
        router.push('/admin/studios');
      }
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setError('Invalid email or password.');
      } else {
        setError('Could not sign in. Try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  const themeStyle: CSSProperties = postBrand
    ? ({
        ['--brand' as string]: postBrand,
        ['--brand-soft' as string]: withAlpha(postBrand, 0.08),
        ['--brand-softer' as string]: withAlpha(postBrand, 0.16),
        ['--brand-onbrand' as string]: '#ffffff',
      } as CSSProperties)
    : {};

  return (
    <main
      className="min-h-screen w-full text-zinc-900 transition-all duration-700 dark:text-zinc-100"
      style={{
        ...themeStyle,
        background: 'rgb(248, 250, 255)',
      }}
    >
      <div className="grid min-h-screen w-full grid-cols-1 lg:grid-cols-[1.1fr,1fr] xl:grid-cols-[1.2fr,1fr]">
        {/* Hero */}
        <section className="relative hidden overflow-hidden bg-neutral-950 text-white lg:flex lg:flex-col lg:justify-between lg:px-12 lg:py-12 xl:px-16">
          {/* Animated Background Image - Left Side Only */}
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 opacity-50"
            style={{
              backgroundImage: 'linear-gradient(to right, rgba(10, 10, 11, 0.2) 0%, rgba(10, 10, 11, 0.8) 100%), url("/platform-bg.png")',
              backgroundSize: 'cover',
              backgroundPosition: 'center',
            }}
          />
          <div className="absolute inset-0 bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-[0.03] brightness-100 contrast-150" />
          
          {/* Floating visual elements */}
          <div className="pointer-events-none absolute inset-0 overflow-hidden">
            <div className="absolute -left-[10%] top-[20%] h-[40%] w-[40%] rounded-full bg-brand-500/15 blur-[120px] animate-pulse-liquid" />
            <div className="absolute -right-[10%] top-[50%] h-[40%] w-[40%] rounded-full bg-sky-500/10 blur-[120px] animate-pulse-liquid" style={{ animationDelay: '3s' }} />
          </div>

          <div className="relative">
            <div className="flex animate-in items-center gap-4" style={{ animationDelay: '100ms' }}>
              <div className="grid h-14 w-14 place-items-center rounded-3xl bg-gradient-to-br from-brand-400 via-brand-primary to-brand-700 text-lg font-black text-white shadow-2xl shadow-brand-500/40 ring-2 ring-white/10">
                1H
              </div>
              <div className="text-2xl font-black tracking-tight text-white">1herosocial.ai</div>
            </div>

            <h1 className="mt-20 max-w-2xl animate-slide-up text-5xl font-black leading-[1.05] tracking-tight text-white xl:text-7xl" style={{ animationDelay: '200ms' }}>
              The AI-run marketing <span className="bg-gradient-to-r from-brand-300 to-sky-300 bg-clip-text text-transparent">OS</span> for fitness studios.
            </h1>
            <p className="mt-8 max-w-lg animate-slide-up text-xl font-medium leading-relaxed text-zinc-300" style={{ animationDelay: '300ms' }}>
              Onboard studios in minutes. Generate campaigns, capture leads in a
              shared inbox, and ship them straight into your spreadsheet.
            </p>
          </div>

          <div className="relative mt-12 grid grid-cols-1 gap-6 animate-in sm:grid-cols-3" style={{ animationDelay: '400ms' }}>
            <Feature icon={<Building2 className="h-5 w-5" />} title="Multi-studio" body="Each studio is its own tenant with branded URLs." />
            <Feature icon={<Inbox className="h-5 w-5" />} title="Lead inbox" body="Form submissions land in Postgres + Sheets, never lost." />
            <Feature icon={<Sparkles className="h-5 w-5" />} title="Per-studio brand" body="Each studio's color and logo render on the public form." />
          </div>
        </section>

        {/* Form - Right Side with Rich Professional Gradient */}
        <section 
          className="relative flex items-center justify-center px-6 py-12 sm:px-10 lg:px-12 xl:px-16"
          style={{
            background: 'linear-gradient(135deg, #f3f0ff 0%, #e0e7ff 50%, #dbeafe 100%)',
          }}
        >
          {/* Enhanced Ambient Glows for Right Side */}
          <div className="absolute inset-0 -z-10 overflow-hidden">
            <div className="absolute left-[-20%] top-[-10%] h-[80%] w-[80%] rounded-full bg-violet-400/20 blur-[120px] animate-pulse-liquid" />
            <div className="absolute bottom-[-20%] right-[-10%] h-[80%] w-[80%] rounded-full bg-sky-400/20 blur-[120px] animate-pulse-liquid" style={{ animationDelay: '2s' }} />
          </div>

          <div className="w-full max-w-md animate-in" style={{ animationDelay: '500ms' }}>
            <div className="mb-12 flex items-center gap-4 lg:hidden">
              <div className="grid h-14 w-14 place-items-center rounded-3xl bg-gradient-to-br from-brand-400 via-brand-primary to-brand-700 text-lg font-black text-white shadow-xl ring-2 ring-white/10">
                1H
              </div>
              <div className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">1herosocial.ai</div>
            </div>

            <div className="glass-container p-1 sm:p-1">
              <div className="rounded-[40px] bg-white/40 p-10 backdrop-blur-3xl dark:bg-neutral-900/40 sm:p-12">
                <div className="mb-10 text-center">
                  <h2 className="text-4xl font-black tracking-tight text-zinc-900 dark:text-white">Welcome back</h2>
                  <p className="mt-4 text-base font-medium text-zinc-500 dark:text-zinc-400">
                    Sign in to manage your fitness studio operations.
                  </p>
                </div>

                <form onSubmit={onSubmit} className="space-y-6">
                  <div className="space-y-2.5">
                    <Label htmlFor="email" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400 dark:text-zinc-500">Email Address</Label>
                    <Input
                      id="email"
                      type="email"
                      autoComplete="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="you@example.com"
                      className="h-14 rounded-[20px] border-white/20 bg-white/50 px-5 text-base shadow-sm backdrop-blur-md focus-visible:ring-offset-0 dark:border-white/5 dark:bg-black/20"
                      suppressHydrationWarning
                    />
                  </div>
                  <div className="space-y-2.5">
                    <div className="flex items-center justify-between px-1">
                      <Label htmlFor="password" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400 dark:text-zinc-500">Password</Label>
                    </div>
                    <Input
                      id="password"
                      type="password"
                      autoComplete="current-password"
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="••••••••"
                      className="h-14 rounded-[20px] border-white/20 bg-white/50 px-5 text-base shadow-sm backdrop-blur-md focus-visible:ring-offset-0 dark:border-white/5 dark:bg-black/20"
                      suppressHydrationWarning
                    />
                  </div>
                  <FieldError message={error ?? undefined} />
                  <Button
                    type="submit"
                    className="h-14 w-full rounded-[24px] text-lg font-black shadow-2xl shadow-brand-500/25"
                    size="lg"
                    loading={submitting}
                    rightIcon={<ArrowRight className="h-5 w-5 transition-transform group-hover:translate-x-2" />}
                    suppressHydrationWarning
                  >
                    Sign in
                  </Button>
                </form>

                <div className="mt-12 flex items-center gap-4 text-[10px] font-black text-zinc-400 dark:text-zinc-500">
                  <div className="h-px flex-1 bg-zinc-100 dark:bg-white/5" />
                  <span className="tracking-[0.2em]">SECURE ACCESS</span>
                  <div className="h-px flex-1 bg-zinc-100 dark:bg-white/5" />
                </div>
              </div>
            </div>

            <p className="mt-8 text-center text-xs font-bold text-zinc-400 dark:text-zinc-500">
              Platform admins and studio managers sign in here.
            </p>
          </div>
        </section>
      </div>
    </main>
  );
}

function Feature({ icon, title, body }: { icon: React.ReactNode; title: string; body: string }) {
  return (
    <div className="group rounded-2xl border border-white/5 bg-white/[0.03] p-5 backdrop-blur-md transition-all duration-300 hover:bg-white/[0.06] hover:shadow-2xl">
      <div className="flex items-center gap-3 text-brand-300 transition-colors group-hover:text-brand-200">
        <div className="rounded-xl bg-brand-500/10 p-2 shadow-inner">
          {icon}
        </div>
        <span className="text-xs font-bold uppercase tracking-widest text-white">{title}</span>
      </div>
      <p className="mt-4 text-sm leading-relaxed text-slate-300 group-hover:text-white transition-colors">{body}</p>
    </div>
  );
}
