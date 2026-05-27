'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Save, CheckCircle, Plug, Globe } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import type { Studio } from '@/lib/types';
import { updateStudioSettings } from '../settings/actions';

interface Props {
  studio: Studio;
}

export function ConnectGoogleAds({ studio }: Props) {
  const router = useRouter();
  const [googleClientId, setGoogleClientId] = useState(studio.googleClientId || '');
  const [googleClientSecret, setGoogleClientSecret] = useState(studio.googleClientSecret || '');
  const [googleDeveloperToken, setGoogleDeveloperToken] = useState(studio.googleDeveloperToken || '');
  const [saving, setSaving] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  // Sync state with updated studio props
  useEffect(() => {
    setGoogleClientId(studio.googleClientId || '');
    setGoogleClientSecret(studio.googleClientSecret || '');
    setGoogleDeveloperToken(studio.googleDeveloperToken || '');
  }, [studio]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSuccess(null);
    setError(null);
    setErrors({});

    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        name: studio.name,
        brandColor: studio.brandColor,
        logoUrl: studio.logoUrl,
        contactEmail: studio.contactEmail,
        active: studio.active,
        googleClientId,
        googleClientSecret,
        googleDeveloperToken,
      });

      if (!result.ok) {
        setError(result.error || 'Failed to save Google Ads settings.');
        setErrors(result.details || {});
      } else {
        setSuccess('Google Ads credentials saved successfully.');
        router.refresh();
      }
    } catch (err: any) {
      setError(err.message || 'An error occurred while saving.');
    } finally {
      setSaving(false);
    }
  }

  async function handleDisconnect() {
    if (!confirm('Are you sure you want to disconnect Google Ads? This will clear your credentials.')) {
      return;
    }
    setDisconnecting(true);
    setError(null);
    setSuccess(null);

    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        name: studio.name,
        brandColor: studio.brandColor,
        logoUrl: studio.logoUrl,
        contactEmail: studio.contactEmail,
        active: studio.active,
        googleClientId: '',
        googleClientSecret: '',
        googleDeveloperToken: '',
      });

      if (!result.ok) {
        setError(result.error || 'Failed to disconnect Google Ads.');
      } else {
        setSuccess('Google Ads credentials cleared successfully.');
        router.refresh();
      }
    } catch (err: any) {
      setError(err.message || 'An error occurred during disconnect.');
    } finally {
      setDisconnecting(false);
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-3">
      {/* Left side: Connected Account / Status */}
      <div className="lg:col-span-2 space-y-6">
        {!studio.googleClientId ? (
          <Card>
            <div className="py-8 text-center">
              <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-2xl text-white shadow-lg" style={{ background: 'linear-gradient(135deg, #4285F4 0%, #34A853 100%)' }}>
                <Globe className="h-6 w-6" />
              </div>
              <p className="text-sm font-bold text-slate-700 dark:text-slate-200">
                No Google Ads account connected
              </p>
              <p className="mt-2 text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto leading-relaxed">
                Connect your Google Ads account to automate campaign tracking and sync incoming lead signals.
              </p>
            </div>
          </Card>
        ) : (
          <Card title="Connected Accounts">
            <div className="space-y-4">
              <div className="flex items-start justify-between rounded-2xl border border-white/10 bg-white/5 p-4">
                <div className="flex gap-4">
                  <div className="grid h-10 w-10 place-items-center rounded-xl bg-blue-500/10 text-blue-600">
                    <Globe className="h-5 w-5" />
                  </div>
                  <div className="min-w-0">
                    <h4 className="text-sm font-black text-zinc-900 dark:text-white">
                      Google Ads Manager
                    </h4>
                    <p className="mt-1 text-xs font-mono text-zinc-500 dark:text-zinc-400 truncate max-w-[280px]">
                      Client ID: {studio.googleClientId}
                    </p>
                    <div className="mt-2 flex items-center gap-2">
                      <span className="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
                        ● Active
                      </span>
                    </div>
                  </div>
                </div>

                <Button
                  variant="ghost"
                  size="sm"
                  className="text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-950/20"
                  loading={disconnecting}
                  onClick={handleDisconnect}
                >
                  Disconnect
                </Button>
              </div>
            </div>
          </Card>
        )}
      </div>

      {/* Right side: Add/Configure Form */}
      <div>
        <Card>
          {/* Google-blue branded header */}
          <div
            className="-mx-5 -mt-5 mb-5 flex items-center gap-3 rounded-t-[inherit] px-5 py-4"
            style={{ background: 'linear-gradient(135deg, #4285F4 0%, #0F9D58 100%)' }}
          >
            <div className="grid h-9 w-9 place-items-center rounded-xl bg-white/20 text-white">
              <Globe className="h-5 w-5" />
            </div>
            <div>
              <div className="text-sm font-black text-white">Connect Google Ads</div>
              <div className="text-[11px] text-white/70">OAuth 2.0 + API developer credentials</div>
            </div>
          </div>
          <form onSubmit={onSubmit} className="space-y-4">
            {success && (
              <div className="flex items-center gap-2 rounded-2xl bg-emerald-500/10 p-4 text-xs font-bold text-emerald-600 dark:text-emerald-400">
                <CheckCircle className="h-4 w-4 shrink-0" />
                {success}
              </div>
            )}

            <div>
              <Label htmlFor="googleClientId">Google Ads Client ID</Label>
              <Input
                id="googleClientId"
                placeholder="e.g. 123456789-abc.apps.googleusercontent.com"
                required
                value={googleClientId}
                onChange={(e) => setGoogleClientId(e.target.value)}
                className="font-mono text-xs mt-1.5"
                invalid={!!errors.googleClientId}
              />
              <FieldHint>The OAuth 2.0 Web Client ID registered in Google Developer Console.</FieldHint>
              <FieldError message={errors.googleClientId} />
            </div>

            <div>
              <Label htmlFor="googleClientSecret">Google Ads Client Secret</Label>
              <Input
                id="googleClientSecret"
                type="password"
                placeholder={studio.googleClientSecret ? "••••••••••••••••" : "e.g. GOCSPX-..."}
                required={!studio.googleClientSecret}
                value={googleClientSecret}
                onChange={(e) => setGoogleClientSecret(e.target.value)}
                className="font-mono text-xs mt-1.5"
                invalid={!!errors.googleClientSecret}
              />
              <FieldHint>The Client Secret matching your Google Web Client ID.</FieldHint>
              <FieldError message={errors.googleClientSecret} />
            </div>

            <div>
              <Label htmlFor="googleDeveloperToken">Google Ads Developer Token</Label>
              <Input
                id="googleDeveloperToken"
                type="password"
                placeholder={studio.googleDeveloperToken ? "••••••••••••••••" : "e.g. AbCdEfGhIjKlMnOpQrStUv"}
                required={!studio.googleDeveloperToken}
                value={googleDeveloperToken}
                onChange={(e) => setGoogleDeveloperToken(e.target.value)}
                className="font-mono text-xs mt-1.5"
                invalid={!!errors.googleDeveloperToken}
              />
              <FieldHint>The Developer Token issued by Google Ads Manager account.</FieldHint>
              <FieldError message={errors.googleDeveloperToken} />
            </div>

            <FieldError message={error ?? undefined} />

            <div className="pt-2">
              <Button
                type="submit"
                className="w-full h-11"
                loading={saving}
                leftIcon={<Save className="h-4 w-4" />}
              >
                {studio.googleClientId ? 'Update Credentials' : 'Connect Google Ads'}
              </Button>
            </div>
          </form>
        </Card>
      </div>
    </div>
  );
}
