'use client';

import { useRouter } from 'next/navigation';
import { ForgotPasswordModal } from './ForgotPasswordModal';
import { useState, useEffect, type CSSProperties } from 'react';
import { ArrowLeft, ArrowRight, Building2, Eye, EyeOff, Inbox, KeyRound, Lock, Mail, ShieldCheck, Sparkles } from 'lucide-react';
import Link from 'next/link';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';
import { withAlpha } from '@/lib/color';
import { sanitizeNextPath } from '@/lib/sessionGuard';
import type { Me } from '@/lib/types';

// The password step returns either a full session (Me) or, when the
// account has TOTP enrolled, this instead (platform-security-hardening
// task 2.3) — no session cookie yet, pendingToken must reach
// /auth/login/2fa or /auth/login/recovery-code within 5 minutes.
interface TwoFactorRequired {
  twoFactorRequired: true;
  pendingToken: string;
}

const noiseSvgDataUri =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160' viewBox='0 0 160 160'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='160' height='160' filter='url(%23n)' opacity='0.38'/%3E%3C/svg%3E";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForgotPassword, setShowForgotPassword] = useState(false);

  // Two-factor step (shown after password when the account has TOTP
  // enrolled) — see TwoFactorRequired above.
  const [step, setStep] = useState<'password' | 'code'>('password');
  const [pendingToken, setPendingToken] = useState('');
  const [code, setCode] = useState('');
  const [useRecoveryCode, setUseRecoveryCode] = useState(false);
  const [rememberBrowser, setRememberBrowser] = useState(false);

  // This page always renders its own fixed dark/purple look, independent of
  // the admin app's light/dark theme preference — so it force-clears the
  // dark class rather than reading or toggling it.
  useEffect(() => {
    document.documentElement.classList.remove('dark');
  }, []);

  const [postBrand, setPostBrand] = useState<string | null>(null);

  function completeLogin(me: Me) {
    // A dead-session redirect (idle timeout, revoked session) tags the
    // login URL with ?next=<path they were on> — read via window.location
    // rather than useSearchParams so this stays a plain client page with
    // no Suspense boundary. Sanitized again here even though the server
    // side already validated it, since nothing stops a user from editing
    // the URL by hand.
    const next = sanitizeNextPath(new URLSearchParams(window.location.search).get('next'));
    if (next) {
      router.push(next);
    } else if (me.studioId) {
      // studio_admin and studio_staff are both scoped to one studio —
      // only super_admin (no studioId) belongs on the bare /admin/studios
      // list, which is super-admin-only and 403s for anyone else.
      setPostBrand(me.studio?.brandColor ?? null);
      router.push(`/admin/studios/${me.studioId}`);
    } else {
      router.push('/admin/studios');
    }
    router.refresh();
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const result = await api<Me | TwoFactorRequired>('/api/v1/auth/login', {
        method: 'POST',
        json: { email, password },
      });
      if ('twoFactorRequired' in result && result.twoFactorRequired) {
        setPendingToken(result.pendingToken);
        setStep('code');
        return;
      }
      completeLogin(result as Me);
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

  async function onSubmitCode(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const path = useRecoveryCode ? '/api/v1/auth/login/recovery-code' : '/api/v1/auth/login/2fa';
      const me = await api<Me>(path, {
        method: 'POST',
        json: { pendingToken, code, rememberBrowser },
      });
      completeLogin(me);
    } catch (err) {
      if (err instanceof ApiError && err.status === 423) {
        setError('Too many wrong codes — locked for 15 minutes. Try again later.');
      } else if (err instanceof ApiError && (err.status === 401 || err.status === 422)) {
        setError(useRecoveryCode ? 'Invalid or already-used recovery code.' : 'Invalid or expired code.');
      } else {
        setError('Could not verify. Try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  function backToPassword() {
    setStep('password');
    setCode('');
    setError(null);
    setUseRecoveryCode(false);
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
      className="relative min-h-screen w-full overflow-hidden bg-[#0d0420] text-white"
      style={themeStyle}
    >
      {/* Single full-bleed background photo shared by the nav and both columns */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{
          backgroundImage: 'linear-gradient(to right, rgba(13, 4, 32, 0.55) 0%, rgba(13, 4, 32, 0.2) 55%, rgba(13, 4, 32, 0.55) 100%), url("/image.png")',
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
        }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-[0.03] brightness-100 contrast-150"
        style={{
          backgroundImage: `url("${noiseSvgDataUri}")`,
          backgroundRepeat: 'repeat',
          backgroundSize: '160px 160px',
        }}
      />

      {/* Top nav bar */}
      <nav className="relative z-30 flex items-center justify-between gap-4 px-6 py-5 lg:px-12 xl:px-16">
        <Link href="/" className="flex items-center gap-3">
          <img src="/logo.png" alt="1herosocial.ai Logo" className="h-9 w-9 object-contain rounded-xl shadow-lg" />
          <span className="text-lg font-black tracking-tight text-white">1herosocial.ai</span>
        </Link>
        <div className="hidden items-center gap-8 text-sm font-semibold text-zinc-300 md:flex">
          <Link href="/" className="transition-colors hover:text-white">Home</Link>
          <Link href="/about" className="transition-colors hover:text-white">About</Link>
          <Link href="/pricing" className="transition-colors hover:text-white">Pricing</Link>
        </div>
      </nav>
      <div className="relative grid min-h-[calc(100vh-76px)] w-full grid-cols-1 lg:grid-cols-[1.1fr,1fr] xl:grid-cols-[1.2fr,1fr]">
        {/* Hero */}
        <section className="relative hidden text-white lg:flex lg:flex-col lg:justify-between lg:px-12 lg:py-12 xl:px-16">
          <div className="relative">
            <div className="flex animate-in items-center gap-2 rounded-full bg-white/10 px-4 py-2 text-xs font-bold text-white backdrop-blur-md w-fit" style={{ animationDelay: '50ms' }}>
              <Sparkles className="h-3.5 w-3.5 text-brand-300" />
              AI Marketing Platform for Fitness Studios
            </div>

            <h1 className="mt-6 max-w-2xl animate-slide-up text-5xl font-black leading-[1.05] tracking-tight text-white xl:text-7xl" style={{ animationDelay: '200ms' }}>
              The AI-run marketing <span className="bg-gradient-to-r from-brand-300 to-sky-300 bg-clip-text text-transparent">OS</span> for fitness studios.
            </h1>
            <p className="mt-6 max-w-lg animate-slide-up text-xl font-medium leading-relaxed text-zinc-300" style={{ animationDelay: '300ms' }}>
              Attract more members, automate marketing, and grow your studio — all in one platform.
            </p>
          </div>

          <div className="relative mt-12 flex flex-col gap-5 animate-in" style={{ animationDelay: '400ms' }}>
            <Feature icon={<Building2 className="h-5 w-5" />} title="Multi-studio" body="Each studio is its own tenant with branded URLs." />
            <Feature icon={<Inbox className="h-5 w-5" />} title="Lead inbox" body="Form submissions land in Postgres + Sheets, never lost." />
            <Feature icon={<Sparkles className="h-5 w-5" />} title="Per-studio brand" body="Each studio's color and logo render on the public form." />
          </div>
        </section>

        {/* Form - Right Side, glass card over the shared hero background */}
        <section className="relative flex items-center justify-center px-6 py-12 sm:px-10 lg:justify-end lg:px-10 xl:px-14">
          <div className="relative w-full max-w-sm animate-in" style={{ animationDelay: '500ms' }}>
            <div className="rounded-[24px] border border-black/10 bg-white/90 p-7 shadow-2xl shadow-black/30 ring-1 ring-black/5 backdrop-blur-2xl sm:p-8">
              <div className="mb-6 flex items-center justify-center gap-2">
                <img src="/web-app-manifest-192x192.png" alt="" className="h-6 w-6 rounded-md" />
                <span className="text-sm font-bold text-zinc-600">
                  Powered by{' '}
                  <a href="https://infaira.ai/" target="_blank" rel="noopener noreferrer" className="hover:underline">
                    Inf<span className="text-brand-600">AI</span>ra
                  </a>
                </span>
              </div>
                {step === 'password' ? (
                  <>
                    <div className="mb-6 text-center">
                      <h2 className="text-2xl font-black tracking-tight text-zinc-900">Welcome back</h2>
                      <p className="mt-2 text-sm font-medium text-zinc-500">
                        Sign in to manage your fitness studio operations.
                      </p>
                    </div>

                    <form onSubmit={onSubmit} className="space-y-4">
                      <div className="space-y-2.5">
                        <Label htmlFor="email" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-500">Email Address</Label>
                        <div className="relative">
                          <Mail className="pointer-events-none absolute left-5 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
                          <Input
                            id="email"
                            type="email"
                            autoComplete="email"
                            required
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            placeholder="you@example.com"
                            className="h-11 rounded-[16px] border-zinc-200 bg-zinc-50 pl-11 pr-4 text-sm text-zinc-900 placeholder:text-zinc-400 shadow-sm focus-visible:ring-offset-0"
                            suppressHydrationWarning
                          />
                        </div>
                      </div>
                      <div className="space-y-2">
                        <div className="flex items-center justify-between px-1">
                          <Label htmlFor="password" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-500">Password</Label>
                          <button
                            type="button"
                            onClick={() => setShowForgotPassword(true)}
                            className="text-xs font-bold text-brand-600 hover:underline"
                          >
                            Forgot password?
                          </button>
                        </div>
                        <div className="relative">
                          <Lock className="pointer-events-none absolute left-5 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
                          <Input
                            id="password"
                            type={showPassword ? 'text' : 'password'}
                            autoComplete="current-password"
                            required
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            placeholder="••••••••"
                            className="h-11 rounded-[16px] border-zinc-200 bg-zinc-50 pl-11 pr-11 text-sm text-zinc-900 placeholder:text-zinc-400 shadow-sm focus-visible:ring-offset-0"
                            suppressHydrationWarning
                          />
                          <button
                            type="button"
                            aria-label={showPassword ? 'Hide password' : 'Show password'}
                            className="absolute right-4 top-1/2 -translate-y-1/2 text-zinc-400 transition hover:text-zinc-700"
                            onClick={() => setShowPassword((value) => !value)}
                          >
                            {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                          </button>
                        </div>
                      </div>
                      <FieldError message={error ?? undefined} />
                      <Button
                        type="submit"
                        className="h-11 w-full rounded-[16px] bg-gradient-to-r from-brand-600 to-violet-600 text-sm font-black shadow-xl shadow-brand-500/25 hover:from-brand-500 hover:to-violet-500"
                        size="lg"
                        loading={submitting}
                        rightIcon={<ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-2" />}
                        suppressHydrationWarning
                      >
                        Sign in
                      </Button>
                    </form>
                  </>
                ) : (
                  <>
                    <button
                      type="button"
                      onClick={backToPassword}
                      className="mb-4 inline-flex items-center gap-1.5 text-xs font-bold text-zinc-500 hover:text-zinc-900"
                    >
                      <ArrowLeft className="h-3.5 w-3.5" /> Back
                    </button>
                    <div className="mb-6 text-center">
                      <div className="mx-auto mb-3 grid h-11 w-11 place-items-center rounded-2xl bg-brand-500/15 text-brand-600">
                        <ShieldCheck className="h-5 w-5" />
                      </div>
                      <h2 className="text-xl font-black tracking-tight text-zinc-900">
                        {useRecoveryCode ? 'Enter a recovery code' : 'Enter your code'}
                      </h2>
                      <p className="mt-2 text-sm font-medium text-zinc-500">
                        {useRecoveryCode
                          ? 'One of the 10 one-time codes you saved when you set up two-factor authentication.'
                          : 'Open your authenticator app and enter the 6-digit code for this account.'}
                      </p>
                    </div>

                    <form onSubmit={onSubmitCode} className="space-y-4">
                      <div className="space-y-2">
                        <Label htmlFor="code" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-500">
                          {useRecoveryCode ? 'Recovery Code' : 'Authentication Code'}
                        </Label>
                        <Input
                          id="code"
                          type="text"
                          inputMode={useRecoveryCode ? 'text' : 'numeric'}
                          autoComplete="one-time-code"
                          autoFocus
                          required
                          value={code}
                          onChange={(e) => setCode(e.target.value)}
                          placeholder={useRecoveryCode ? 'xxxx-xxxx' : '123456'}
                          className="h-11 rounded-[16px] border-zinc-200 bg-zinc-50 px-4 text-center text-lg tracking-[0.3em] text-zinc-900 placeholder:text-zinc-400 shadow-sm focus-visible:ring-offset-0"
                          suppressHydrationWarning
                        />
                      </div>

                      <label className="flex cursor-pointer items-center gap-2.5 px-1 text-sm font-medium text-zinc-600">
                        <input
                          type="checkbox"
                          checked={rememberBrowser}
                          onChange={(e) => setRememberBrowser(e.target.checked)}
                          className="h-4 w-4 rounded border-zinc-400 text-brand-600 focus:ring-brand-500"
                        />
                        Remember this browser for 14 days
                      </label>

                      <FieldError message={error ?? undefined} />
                      <Button
                        type="submit"
                        className="h-11 w-full rounded-[16px] bg-gradient-to-r from-brand-600 to-violet-600 text-sm font-black shadow-xl shadow-brand-500/25 hover:from-brand-500 hover:to-violet-500"
                        size="lg"
                        loading={submitting}
                        rightIcon={<ArrowRight className="h-5 w-5 transition-transform group-hover:translate-x-2" />}
                        suppressHydrationWarning
                      >
                        Verify
                      </Button>

                      <button
                        type="button"
                        onClick={() => {
                          setUseRecoveryCode((v) => !v);
                          setCode('');
                          setError(null);
                        }}
                        className="flex w-full items-center justify-center gap-1.5 text-xs font-bold text-brand-600 hover:underline"
                      >
                        <KeyRound className="h-3.5 w-3.5" />
                        {useRecoveryCode ? 'Use my authenticator app instead' : "Lost your phone? Use a recovery code"}
                      </button>
                    </form>
                  </>
                )}

                <div className="mt-6 flex items-center gap-4 text-[10px] font-black text-zinc-400">
                  <div className="h-px flex-1 bg-black/10" />
                  <span className="tracking-[0.2em]">SECURE ACCESS</span>
                  <div className="h-px flex-1 bg-black/10" />
                </div>
            </div>
          </div>
        </section>
      </div>
      <ForgotPasswordModal open={showForgotPassword} onClose={() => setShowForgotPassword(false)} />
    </main>
  );
}

function Feature({ icon, title, body }: { icon: React.ReactNode; title: string; body: string }) {
  return (
    <div className="group flex items-start gap-4">
      <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-brand-500/20 text-brand-300 shadow-inner transition-colors group-hover:bg-brand-500/30 group-hover:text-brand-200">
        {icon}
      </div>
      <div>
        <div className="text-base font-bold text-white">{title}</div>
        <p className="mt-0.5 text-sm leading-relaxed text-zinc-300">{body}</p>
      </div>
    </div>
  );
}
