'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Save, CheckCircle, Plug, Globe } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import type { Studio, ChannelAccount } from '@/lib/types';
import { updateStudioSettings } from '../settings/actions';
import { api } from '@/lib/api';

interface Props {
  studio: Studio;
  channels: ChannelAccount[];
  showToast: (msg: string) => void;
}

export function ConnectGoogleAds({ studio, channels, showToast }: Props) {
  const router = useRouter();
  const [googleClientId, setGoogleClientId] = useState(studio.googleClientId || '');
  const [googleClientSecret, setGoogleClientSecret] = useState(studio.googleClientSecret || '');
  const [googleDeveloperToken, setGoogleDeveloperToken] = useState(studio.googleDeveloperToken || '');
  const [googleCustomerId, setGoogleCustomerId] = useState(channels[0]?.externalId || '');
  const [googleLoginCustomerId, setGoogleLoginCustomerId] = useState(channels[0]?.parentId || '');
  const [saving, setSaving] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const [authenticating, setAuthenticating] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const hasCredentials = !!studio.googleClientId;
  const isOAuthenticated = channels.length > 0;

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      if (event.data?.type !== 'google_ads_oauth_success') return;
      setAuthenticating(false);
      setSuccess('Google Ads connected successfully.');
      showToast('Google Ads connected successfully.');
      router.refresh();
    };

    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [router, showToast]);

  async function saveGoogleCredentials() {
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
      setErrors(result.details || {});
      throw new Error(result.error || 'Failed to save Google Ads settings.');
    }
  }

  async function handleOAuthLogin() {
    const normalizedCustomerId = googleCustomerId.trim().replace(/[\s-]/g, '');
    const normalizedLoginCustomerId = googleLoginCustomerId.trim().replace(/[\s-]/g, '');
    if (!/^\d+$/.test(normalizedCustomerId)) {
      setError('Google Ads Customer ID must contain digits only.');
      return;
    }
    if (normalizedLoginCustomerId && !/^\d+$/.test(normalizedLoginCustomerId)) {
      setError('Google Ads Login Customer ID must contain digits only.');
      return;
    }
    if (!googleClientId.trim() || !googleClientSecret.trim() || !googleDeveloperToken.trim()) {
      setError('Google Client ID, Client Secret, and Developer Token are required before Google login.');
      return;
    }

    const popup = window.open('', 'google_ads_oauth', 'width=520,height=720,noopener=false,noreferrer=false');
    if (popup) {
      popup.document.write('<p style="font-family: system-ui, sans-serif; padding: 24px;">Opening Google sign-in...</p>');
    }

    setAuthenticating(true);
    setError(null);
    setErrors({});
    try {
      await saveGoogleCredentials();
      const loginUrl = new URL(`/api/v1/studios/${studio.id}/google-oauth/login`, window.location.origin);
      loginUrl.searchParams.set('customerId', normalizedCustomerId);
      if (normalizedLoginCustomerId) {
        loginUrl.searchParams.set('loginCustomerId', normalizedLoginCustomerId);
      }
      const data = await api<{ url: string }>(loginUrl.pathname + loginUrl.search);
      if (popup) {
        const popupTimer = window.setInterval(() => {
          if (!popup.closed) return;
          window.clearInterval(popupTimer);
          setAuthenticating(false);
          router.refresh();
        }, 1000);
        popup.location.href = data.url;
      } else {
        window.location.href = data.url;
      }
    } catch (err: any) {
      popup?.close();
      setError(err.message || 'Error initiating Google login');
      setAuthenticating(false);
    }
  }

  // Sync state with updated studio props
  useEffect(() => {
    setGoogleClientId(studio.googleClientId || '');
    setGoogleClientSecret(studio.googleClientSecret || '');
    setGoogleDeveloperToken(studio.googleDeveloperToken || '');
    setGoogleCustomerId(channels[0]?.externalId || '');
    setGoogleLoginCustomerId(channels[0]?.parentId || '');
  }, [studio, channels]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSuccess(null);
    setError(null);
    setErrors({});

    const normalizedCustomerId = googleCustomerId.trim().replace(/[\s-]/g, '');
    const normalizedLoginCustomerId = googleLoginCustomerId.trim().replace(/[\s-]/g, '');

    if (normalizedCustomerId && !/^\d+$/.test(normalizedCustomerId)) {
      setError('Google Ads Customer ID must contain digits only.');
      setSaving(false);
      return;
    }
    if (normalizedLoginCustomerId && !/^\d+$/.test(normalizedLoginCustomerId)) {
      setError('Google Ads Login Customer ID must contain digits only.');
      setSaving(false);
      return;
    }

    try {
      await saveGoogleCredentials();

      const channel = channels[0];
      if (channel) {
        await api(`/api/v1/studios/${studio.id}/messaging/channels/${channel.id}`, {
          method: 'PUT',
          json: {
            externalId: normalizedCustomerId,
            parentId: normalizedLoginCustomerId,
            displayHandle: `Google Ads ${normalizedCustomerId}`,
          },
        });
      }

      setSuccess('Google Ads credentials saved successfully.');
      showToast('Google Ads channel credentials saved successfully.');
      router.refresh();
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
        await Promise.all(channels.map(async (channel) => {
          await api(`/api/v1/studios/${studio.id}/messaging/channels/${channel.id}`, {
            method: 'DELETE',
          });
        }));
        setSuccess('Google Ads credentials cleared successfully.');
        showToast('Google Ads channel disconnected successfully.');
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
                    {isOAuthenticated && (
                      <p className="mt-1 text-xs font-mono text-zinc-500 dark:text-zinc-400 truncate max-w-[280px]">
                        Customer ID: {channels[0]?.externalId}
                      </p>
                    )}
                    {isOAuthenticated && channels[0]?.parentId && (
                      <p className="mt-1 text-xs font-mono text-zinc-500 dark:text-zinc-400 truncate max-w-[280px]">
                        Login Customer ID: {channels[0]?.parentId}
                      </p>
                    )}
                    <div className="mt-2 flex items-center gap-2">
                      {isOAuthenticated ? (
                        <span className="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
                          ● Active
                        </span>
                      ) : (
                        <Button
                          size="sm"
                          variant="outline"
                          loading={authenticating}
                          onClick={handleOAuthLogin}
                          className="h-7 text-xs border-blue-500/20 bg-blue-50/50 text-blue-700 hover:bg-blue-100 hover:text-blue-800 dark:bg-blue-500/10 dark:text-blue-400"
                        >
                          <Globe className="mr-1.5 h-3 w-3" />
                          Log in with Google
                        </Button>
                      )}
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

            <div>
              <Label htmlFor="googleCustomerId">Google Ads Customer ID</Label>
              <Input
                id="googleCustomerId"
                placeholder="e.g. 123-456-7890"
                value={googleCustomerId}
                onChange={(e) => setGoogleCustomerId(e.target.value)}
                className="font-mono text-xs mt-1.5"
              />
              <FieldHint>The account ID that should receive campaigns. Hyphens are optional.</FieldHint>
            </div>

            <div>
              <Label htmlFor="googleLoginCustomerId">Google Ads Login Customer ID</Label>
              <Input
                id="googleLoginCustomerId"
                placeholder="e.g. 987-654-3210 (manager account, optional)"
                value={googleLoginCustomerId}
                onChange={(e) => setGoogleLoginCustomerId(e.target.value)}
                className="font-mono text-xs mt-1.5"
              />
              <FieldHint>Only needed when the customer account is accessed through a manager account.</FieldHint>
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
              {!isOAuthenticated && (
                <Button
                  type="button"
                  variant="outline"
                  className="mt-3 w-full h-11"
                  loading={authenticating}
                  onClick={handleOAuthLogin}
                  leftIcon={<Plug className="h-4 w-4" />}
                >
                  Log in with Google
                </Button>
              )}
            </div>
          </form>
        </Card>
      </div>
    </div>
  );
}
