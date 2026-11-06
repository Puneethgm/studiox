'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import QRCode from 'qrcode';
import { Apple, Check, Copy, KeyRound, ShieldCheck, Smartphone } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

interface SetupRes {
  secret: string;
  uri: string;
}

interface RecoveryCodesRes {
  recoveryCodes: string[];
}

// Official, stable app listing URLs for Google Authenticator — free,
// widely used, no account required. Offered as the default recommendation
// for anyone who doesn't already have an authenticator app; any TOTP app
// (Authy, 1Password, Microsoft Authenticator, etc.) works equally well
// with the secret/QR below once installed.
const AUTHENTICATOR_APP_LINKS = {
  ios: 'https://apps.apple.com/app/google-authenticator/id388497605',
  android: 'https://play.google.com/store/apps/details?id=com.google.android.apps.authenticator2',
} as const;

// Mandatory enrollment screen — reached only via the redirect in
// src/lib/auth.ts's serverFetch when the API returns
// totp_enrollment_required (platform-security-hardening task 2.9: no
// grace period for super_admin/studio_admin, 14-day grace for
// studio_staff). The QR is rendered entirely client-side via the `qrcode`
// package (already used by apps/tg-web/apps/wa-web for their own QR
// logins) — the secret never leaves the browser or reaches a third-party
// image service. Manual entry is kept as a fallback for apps/devices that
// can't scan.
export default function TwoFactorSetupPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [setupError, setSetupError] = useState<string | null>(null);
  const [secret, setSecret] = useState('');
  const [qrDataUrl, setQrDataUrl] = useState('');
  const [copied, setCopied] = useState(false);
  const [iosAppQr, setIosAppQr] = useState('');
  const [androidAppQr, setAndroidAppQr] = useState('');
  const [haveApp, setHaveApp] = useState(false);

  const [code, setCode] = useState('');
  const [confirming, setConfirming] = useState(false);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
  const [savedCodes, setSavedCodes] = useState(false);
  const [codesCopied, setCodesCopied] = useState(false);

  useEffect(() => {
    let cancelled = false;
    QRCode.toDataURL(AUTHENTICATOR_APP_LINKS.ios, { width: 160, margin: 1 }).then((url) => {
      if (!cancelled) setIosAppQr(url);
    });
    QRCode.toDataURL(AUTHENTICATOR_APP_LINKS.android, { width: 160, margin: 1 }).then((url) => {
      if (!cancelled) setAndroidAppQr(url);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    api<SetupRes>('/api/v1/auth/2fa/setup', { method: 'POST' })
      .then(async (res) => {
        if (cancelled) return;
        setSecret(res.secret);
        const dataUrl = await QRCode.toDataURL(res.uri, { width: 240, margin: 1 });
        if (!cancelled) setQrDataUrl(dataUrl);
      })
      .catch(() => {
        if (!cancelled) setSetupError('Could not start enrollment. Try reloading the page.');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // navigator.clipboard only exists in a secure context (HTTPS or
  // localhost) — accessing this page over plain HTTP via an IP address
  // (as in local/dev testing) leaves it undefined, so fall back to the
  // older execCommand('copy') path via a temporary textarea, which works
  // regardless of context security.
  function copyText(text: string): boolean {
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(text).catch(() => copyTextFallback(text));
      return true;
    }
    return copyTextFallback(text);
  }

  function copyTextFallback(text: string): boolean {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    let ok = false;
    try {
      ok = document.execCommand('copy');
    } catch {
      ok = false;
    }
    document.body.removeChild(textarea);
    return ok;
  }

  function copySecret() {
    if (copyText(secret)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  async function onConfirm(e: React.FormEvent) {
    e.preventDefault();
    setConfirming(true);
    setConfirmError(null);
    try {
      const res = await api<RecoveryCodesRes>('/api/v1/auth/2fa/confirm', {
        method: 'POST',
        json: { code },
      });
      setRecoveryCodes(res.recoveryCodes);
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        setConfirmError('Wrong code — check your authenticator app and try again.');
      } else {
        setConfirmError('Could not confirm. Try again.');
      }
    } finally {
      setConfirming(false);
    }
  }

  function copyAllCodes() {
    if (!recoveryCodes) return;
    if (copyText(recoveryCodes.join('\n'))) {
      setCodesCopied(true);
      setTimeout(() => setCodesCopied(false), 2000);
    }
  }

  function finish() {
    router.push('/admin/studios');
    router.refresh();
  }

  return (
    <div className="mx-auto max-w-lg py-10">
      <div className="mb-8 text-center">
        <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-2xl bg-brand-500/10 text-brand-600 dark:text-brand-400">
          <ShieldCheck className="h-7 w-7" />
        </div>
        <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">
          Set up two-factor authentication
        </h1>
        <p className="mt-2 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          Required for this account before you can continue.
        </p>
      </div>

      {recoveryCodes ? (
        <Card title="Save your recovery codes" subtitle="Shown once — you won't be able to see these again.">
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-2 rounded-xl bg-zinc-50 p-4 font-mono text-sm dark:bg-zinc-800">
              {recoveryCodes.map((c) => (
                <div key={c}>{c}</div>
              ))}
            </div>
            <Button
              variant="secondary"
              onClick={copyAllCodes}
              leftIcon={codesCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              className="w-full"
            >
              {codesCopied ? 'Copied!' : 'Copy all codes'}
            </Button>
            <label className="flex cursor-pointer items-center gap-2.5 text-sm font-medium text-zinc-600 dark:text-zinc-300">
              <input
                type="checkbox"
                checked={savedCodes}
                onChange={(e) => setSavedCodes(e.target.checked)}
                className="h-4 w-4 rounded border-zinc-300 text-brand-600 focus:ring-brand-500 dark:border-zinc-600"
              />
              I've saved these codes somewhere safe
            </label>
            <Button onClick={finish} disabled={!savedCodes} className="w-full">
              Continue
            </Button>
          </div>
        </Card>
      ) : !haveApp ? (
        <Card
          title="0. Get an authenticator app"
          subtitle="Free, works on iPhone and Android — Google Authenticator recommended."
        >
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="flex flex-col items-center rounded-xl border border-zinc-100 p-4 text-center dark:border-zinc-800">
              <div className="mb-2 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-zinc-500">
                <Apple className="h-3.5 w-3.5" /> iPhone / iPad
              </div>
              {iosAppQr ? (
                // eslint-disable-next-line @next/next/no-img-element -- client-generated data: URL
                <img src={iosAppQr} alt="Scan to install Google Authenticator on iOS" width={160} height={160} className="rounded-lg" />
              ) : (
                <div className="h-[160px] w-[160px] animate-pulse rounded-lg bg-zinc-100 dark:bg-zinc-800" />
              )}
              <a
                href={AUTHENTICATOR_APP_LINKS.ios}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 text-xs font-bold text-brand-600 hover:underline dark:text-brand-400"
              >
                Open in App Store
              </a>
            </div>
            <div className="flex flex-col items-center rounded-xl border border-zinc-100 p-4 text-center dark:border-zinc-800">
              <div className="mb-2 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-zinc-500">
                <Smartphone className="h-3.5 w-3.5" /> Android
              </div>
              {androidAppQr ? (
                // eslint-disable-next-line @next/next/no-img-element -- client-generated data: URL
                <img src={androidAppQr} alt="Scan to install Google Authenticator on Android" width={160} height={160} className="rounded-lg" />
              ) : (
                <div className="h-[160px] w-[160px] animate-pulse rounded-lg bg-zinc-100 dark:bg-zinc-800" />
              )}
              <a
                href={AUTHENTICATOR_APP_LINKS.android}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 text-xs font-bold text-brand-600 hover:underline dark:text-brand-400"
              >
                Open in Play Store
              </a>
            </div>
          </div>
          <p className="mt-4 text-xs text-zinc-400">
            Scan the QR for your phone's platform with its camera app to go
            straight to the install page. Any authenticator app works, not
            just Google Authenticator.
          </p>
          <Button onClick={() => setHaveApp(true)} className="mt-6 w-full">
            Continue — I have an authenticator app installed
          </Button>
        </Card>
      ) : (
        <Card title="1. Scan with your authenticator app" subtitle="Google Authenticator, Authy, 1Password, Microsoft Authenticator, etc.">
          {loading ? (
            <p className="text-sm text-zinc-500 dark:text-zinc-400">Generating your secret…</p>
          ) : setupError ? (
            <FieldError message={setupError} />
          ) : (
            <>
              {qrDataUrl && (
                <div className="mb-6 flex justify-center">
                  {/* eslint-disable-next-line @next/next/no-img-element -- client-generated data: URL, not a static asset */}
                  <img
                    src={qrDataUrl}
                    alt="Scan this QR code with your authenticator app"
                    className="rounded-xl border border-zinc-100 bg-white p-3 dark:border-zinc-800"
                    width={240}
                    height={240}
                  />
                </div>
              )}
              <div className="space-y-2">
                <Label className="text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400">
                  Can't scan? Enter this key manually
                </Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 overflow-x-auto rounded-xl bg-zinc-50 px-4 py-3 font-mono text-sm dark:bg-zinc-800">
                    {secret}
                  </code>
                  <Button type="button" variant="secondary" onClick={copySecret} className="shrink-0 px-3">
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>
                <p className="text-xs text-zinc-400">
                  In your app: Add account → Enter a setup key → paste this code (Time-based).
                </p>
              </div>

              <form onSubmit={onConfirm} className="mt-6 space-y-4 border-t border-zinc-100 pt-6 dark:border-zinc-800">
                <Label htmlFor="confirm-code" className="text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400">
                  2. Enter the 6-digit code it shows
                </Label>
                <Input
                  id="confirm-code"
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  required
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="123456"
                  className="h-14 text-center text-xl tracking-[0.3em]"
                />
                <FieldError message={confirmError ?? undefined} />
                <Button type="submit" loading={confirming} leftIcon={<KeyRound className="h-4 w-4" />} className="w-full">
                  Confirm and enable
                </Button>
              </form>
            </>
          )}
        </Card>
      )}
    </div>
  );
}
