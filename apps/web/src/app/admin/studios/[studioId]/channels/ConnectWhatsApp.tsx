'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Plug } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export function ConnectWhatsApp({ studioId }: { studioId: string }) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [wabaId, setWabaId] = useState('');
  const [phoneNumberId, setPhoneNumberId] = useState('');
  const [displayPhone, setDisplayPhone] = useState('');
  const [accessToken, setAccessToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const getErrorMessage = (msg: string): string => {
    if (msg.includes('already connected')) return '🔴 This WhatsApp account is already connected to another studio. Disconnect it there first.';
    if (msg.includes('401')) return '🔴 Access token expired or invalid. Renew your token in Meta Business Manager.';
    if (msg.includes('403')) return '🔴 Token missing WhatsApp permissions. Check token scopes in Meta.';
    if (msg.includes('404')) return '🔴 Phone number not found in account. Verify Phone Number ID is correct.';
    if (msg.includes('invalid')) return '🔴 Invalid credentials. Double-check WABA ID, Phone Number ID, and token.';
    return `🔴 ${msg}`;
  };

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/whatsapp`, {
        method: 'POST',
        json: { wabaId, phoneNumberId, displayPhone, accessToken },
      });
      // Reset + refresh.
      setWabaId('');
      setPhoneNumberId('');
      setDisplayPhone('');
      setAccessToken('');
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) setError(getErrorMessage(err.message));
      else setError(getErrorMessage('Could not connect.'));
    } finally {
      setSubmitting(false);
    }
  }

  if (!mounted) {
    return (
      <Card id="connect-whatsapp" title="Connect WhatsApp" subtitle="Direct via Meta WhatsApp Cloud API. Token is encrypted at rest.">
        <div className="space-y-3 py-2">
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg bg-slate-100 dark:bg-slate-800" />
        </div>
      </Card>
    );
  }

  return (
    <Card id="connect-whatsapp" title="Connect WhatsApp" subtitle="Direct via Meta WhatsApp Cloud API. Token is encrypted at rest.">
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <Label htmlFor="wabaId">WhatsApp Business Account (WABA) ID</Label>
          <Input
            id="wabaId"
            placeholder="987654321098765 (from Meta API Setup)"
            required
            value={wabaId}
            onChange={(e) => setWabaId(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
            aria-describedby="wabaId-hint"
          />
          <FieldHint id="wabaId-hint">Find this in Meta → WhatsApp → API Setup page</FieldHint>
        </div>
        <div>
          <Label htmlFor="phoneNumberId">Phone Number ID</Label>
          <Input
            id="phoneNumberId"
            placeholder="123456789012345 (from Meta API Setup)"
            required
            value={phoneNumberId}
            onChange={(e) => setPhoneNumberId(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
            aria-describedby="phoneNumberId-hint"
          />
          <FieldHint id="phoneNumberId-hint">From the WhatsApp → API Setup page in your Meta App.</FieldHint>
        </div>
        <div>
          <Label htmlFor="displayPhone">Display phone number</Label>
          <Input
            id="displayPhone"
            placeholder="+1 555 645 5341 (e.g. your business number)"
            required
            value={displayPhone}
            onChange={(e) => setDisplayPhone(e.target.value)}
            suppressHydrationWarning
            aria-describedby="displayPhone-hint"
          />
          <FieldHint id="displayPhone-hint">Shown in the inbox header. Just for humans.</FieldHint>
        </div>
        <div>
          <Label htmlFor="accessToken">Access token</Label>
          <Input
            id="accessToken"
            type="password"
            required
            placeholder="EAAGxxxxxxxxxxxx (System User token recommended)"
            value={accessToken}
            onChange={(e) => setAccessToken(e.target.value)}
            suppressHydrationWarning
            className="font-mono text-xs"
            aria-describedby="accessToken-hint"
          />
          <FieldHint id="accessToken-hint">Encrypted at rest with AES-256-GCM. Never written to logs.</FieldHint>
        </div>
        <FieldError message={error ?? undefined} />
        <Button 
          type="submit" 
          className="w-full h-11" 
          loading={submitting} 
          leftIcon={<Plug className="h-4 w-4" />}
          aria-label="Connect WhatsApp Business Account"
        >
          Connect WhatsApp
        </Button>
        <p className="text-xs text-center text-slate-500 dark:text-slate-400">
          <a href="/docs/meta-whatsapp" className="text-blue-600 dark:text-blue-400 hover:underline">
            Setup guide →
          </a>
        </p>
      </form>
    </Card>
  );
}
