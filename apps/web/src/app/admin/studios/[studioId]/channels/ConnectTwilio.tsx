'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Plug } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export function ConnectTwilio({ studioId }: { studioId: string }) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [accountSid, setAccountSid] = useState('');
  const [authToken, setAuthToken] = useState('');
  const [phoneNumber, setPhoneNumber] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const getErrorMessage = (msg: string): string => {
    if (msg.includes('already connected')) return '🔴 This Twilio account is already connected to another studio. Disconnect it there first.';
    if (msg.includes('invalid')) return '🔴 Invalid credentials. Double-check Account SID and Auth Token.';
    return `🔴 ${msg}`;
  };

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/twilio`, {
        method: 'POST',
        json: { accountSid, authToken, phoneNumber },
      });
      // Reset + refresh.
      setAccountSid('');
      setAuthToken('');
      setPhoneNumber('');
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
      <Card id="connect-twilio" title="Connect Twilio SMS" subtitle="Connect your Twilio account to send and receive SMS messages.">
        <div className="space-y-3 py-2">
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg bg-slate-100 dark:bg-slate-800" />
        </div>
      </Card>
    );
  }

  return (
    <Card id="connect-twilio" title="Connect Twilio SMS" subtitle="Connect your Twilio account to send and receive SMS messages. Tokens are encrypted at rest.">
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <Label htmlFor="accountSid">Account SID</Label>
          <Input
            id="accountSid"
            placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxx"
            required
            value={accountSid}
            onChange={(e) => setAccountSid(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
            aria-describedby="accountSid-hint"
          />
          <FieldHint id="accountSid-hint">Find this in your Twilio Console Dashboard</FieldHint>
        </div>
        <div>
          <Label htmlFor="authToken">Auth Token</Label>
          <Input
            id="authToken"
            type="password"
            placeholder="xxxxxxxxxxxxxxxxxxxxxxxx"
            required
            value={authToken}
            onChange={(e) => setAuthToken(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
            aria-describedby="authToken-hint"
          />
          <FieldHint id="authToken-hint">Encrypted at rest with AES-256-GCM. Never written to logs.</FieldHint>
        </div>
        <div>
          <Label htmlFor="phoneNumber">Twilio Phone Number</Label>
          <Input
            id="phoneNumber"
            placeholder="+15556455341"
            required
            value={phoneNumber}
            onChange={(e) => setPhoneNumber(e.target.value)}
            suppressHydrationWarning
            aria-describedby="phoneNumber-hint"
          />
          <FieldHint id="phoneNumber-hint">The active phone number purchased in Twilio.</FieldHint>
        </div>
        <FieldError message={error ?? undefined} />
        <Button 
          type="submit" 
          className="w-full h-11" 
          loading={submitting} 
          leftIcon={<Plug className="h-4 w-4" />}
          aria-label="Connect Twilio SMS"
        >
          Connect Twilio
        </Button>
      </form>
    </Card>
  );
}
