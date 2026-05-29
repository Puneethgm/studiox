'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Twitter } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export function ConnectX({ studioId, onSuccess, showToast }: { studioId: string; onSuccess: () => void; showToast: (msg: string) => void }) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [consumerKey, setConsumerKey] = useState('');
  const [consumerSecret, setConsumerSecret] = useState('');
  const [accessToken, setAccessToken] = useState('');
  const [accessTokenSecret, setAccessTokenSecret] = useState('');
  const [xHandle, setXHandle] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const getErrorMessage = (msg: string): string => {
    if (msg.includes('already connected')) return '🔴 This X account is already connected to another studio.';
    return `🔴 ${msg}`;
  };

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/x`, {
        method: 'POST',
        json: { 
          consumer_key: consumerKey, 
          consumer_secret: consumerSecret, 
          access_token: accessToken, 
          access_token_secret: accessTokenSecret, 
          x_handle: xHandle 
        },
      });
      setConsumerKey('');
      setConsumerSecret('');
      setAccessToken('');
      setAccessTokenSecret('');
      setXHandle('');
      onSuccess();
      showToast('X (Twitter) DM channel connected successfully.');
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
      <Card id="connect-x" title="Connect X (Twitter) DMs" subtitle="Connect your X App to receive and send DMs.">
        <div className="space-y-3 py-2">
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg bg-slate-100 dark:bg-slate-800" />
        </div>
      </Card>
    );
  }

  return (
    <Card id="connect-x" title="Connect X (Twitter) DMs" subtitle="Enter your X Developer OAuth 1.0a Keys. Tokens are encrypted at rest. NOTE: X requires the Basic API Tier to send DMs.">
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <Label htmlFor="consumerKey">Consumer Key (API Key)</Label>
          <Input
            id="consumerKey"
            placeholder="e.g. mfnwJ9..."
            required
            value={consumerKey}
            onChange={(e) => setConsumerKey(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
          />
        </div>
        <div>
          <Label htmlFor="consumerSecret">Consumer Secret (API Secret)</Label>
          <Input
            id="consumerSecret"
            type="password"
            required
            value={consumerSecret}
            onChange={(e) => setConsumerSecret(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
          />
        </div>
        <div>
          <Label htmlFor="accessToken">Access Token</Label>
          <Input
            id="accessToken"
            placeholder="e.g. 206025709..."
            required
            value={accessToken}
            onChange={(e) => setAccessToken(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
          />
        </div>
        <div>
          <Label htmlFor="accessTokenSecret">Access Token Secret</Label>
          <Input
            id="accessTokenSecret"
            type="password"
            required
            value={accessTokenSecret}
            onChange={(e) => setAccessTokenSecret(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
          />
        </div>
        <div>
          <Label htmlFor="xHandle">X Handle (without @)</Label>
          <Input
            id="xHandle"
            placeholder="e.g. PuneethGMt2"
            required
            value={xHandle}
            onChange={(e) => setXHandle(e.target.value)}
            suppressHydrationWarning
            aria-describedby="xHandle-hint"
          />
          <FieldHint id="xHandle-hint">The handle associated with these API keys.</FieldHint>
        </div>
        
        <FieldError message={error ?? undefined} />
        
        <Button 
          type="submit" 
          className="w-full h-11" 
          loading={submitting} 
          leftIcon={<Twitter className="h-4 w-4" />}
          aria-label="Connect X Account"
        >
          Connect X Account
        </Button>
      </form>
    </Card>
  );
}
