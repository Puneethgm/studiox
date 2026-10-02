'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Plug } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export function ConnectEmailSMTP({ studioId, showToast }: { studioId: string; showToast: (msg: string) => void }) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [host, setHost] = useState('');
  const [port, setPort] = useState('587');
  const [user, setUser] = useState('');
  const [password, setPassword] = useState('');
  const [from, setFrom] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const getErrorMessage = (msg: string): string => {
    if (msg.includes('already connected')) return '🔴 This email account is already connected to another studio. Disconnect it there first.';
    if (msg.includes('invalid SMTP credentials')) return '🔴 Invalid SMTP credentials. Double-check host, port, username, and password.';
    return `🔴 ${msg}`;
  };

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/email`, {
        method: 'POST',
        json: { host, port: Number(port), user, password, from },
      });
      setHost('');
      setPort('587');
      setUser('');
      setPassword('');
      setFrom('');
      showToast('Email account connected successfully.');
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
      <Card id="connect-email-smtp" title="Connect Email" subtitle="Connect your own outbound email account.">
        <div className="space-y-3 py-2">
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg bg-slate-100 dark:bg-slate-800" />
        </div>
      </Card>
    );
  }

  return (
    <Card
      id="connect-email-smtp"
      title="Connect Email"
      subtitle="Connect your own outbound email account (SMTP). Credentials are verified and encrypted at rest. Outbound-only — this is not an inbox."
    >
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div className="col-span-2">
            <Label htmlFor="smtpHost">Host</Label>
            <Input
              id="smtpHost"
              placeholder="smtp.gmail.com"
              required
              value={host}
              onChange={(e) => setHost(e.target.value)}
              suppressHydrationWarning
            />
          </div>
          <div>
            <Label htmlFor="smtpPort">Port</Label>
            <Input
              id="smtpPort"
              type="number"
              placeholder="587"
              required
              value={port}
              onChange={(e) => setPort(e.target.value)}
              suppressHydrationWarning
            />
          </div>
          <div>
            <Label htmlFor="smtpFrom">From address</Label>
            <Input
              id="smtpFrom"
              type="email"
              placeholder="studio@example.com"
              required
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              suppressHydrationWarning
            />
          </div>
          <div className="col-span-2">
            <Label htmlFor="smtpUser">Username</Label>
            <Input
              id="smtpUser"
              placeholder="studio@example.com"
              required
              value={user}
              onChange={(e) => setUser(e.target.value)}
              suppressHydrationWarning
            />
          </div>
          <div className="col-span-2">
            <Label htmlFor="smtpPassword">Password</Label>
            <Input
              id="smtpPassword"
              type="password"
              placeholder="App password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="font-mono text-xs"
              suppressHydrationWarning
              aria-describedby="smtpPassword-hint"
            />
            <FieldHint id="smtpPassword-hint">
              For Gmail, use an App Password (Google Account &gt; Security &gt; App Passwords), not your regular password.
            </FieldHint>
          </div>
        </div>
        <FieldError message={error ?? undefined} />
        <Button
          type="submit"
          className="w-full h-11"
          loading={submitting}
          leftIcon={<Plug className="h-4 w-4" />}
          aria-label="Connect Email"
        >
          Connect Email
        </Button>
      </form>
    </Card>
  );
}
