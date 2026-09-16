'use client';

import { useEffect, useState, useTransition } from 'react';
import { Building2, Link2, Unlink, Check } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { EmptyState } from '@/components/ui/EmptyState';
import { connectStudioToProvider, disconnectStudio, getStudioConnection } from './actions';
import type { CrmProvider } from './types';
import type { Studio } from '@/lib/types';

export function StudioConnectionCard({ studios, activeProviders }: { studios: Studio[]; activeProviders: CrmProvider[] }) {
  const [studioId, setStudioId] = useState(studios[0]?.id ?? '');
  const [connectedProviderId, setConnectedProviderId] = useState<string | null | undefined>(undefined); // undefined = loading
  const [selectedProviderId, setSelectedProviderId] = useState(activeProviders[0]?.id ?? '');
  const [credentials, setCredentials] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  useEffect(() => {
    if (!studioId) return;
    setConnectedProviderId(undefined);
    setError(null);
    setSuccess(null);
    getStudioConnection(studioId).then((result) => {
      if (result.ok) {
        setConnectedProviderId(result.data.connected ? result.data.connection?.crmProviderId ?? null : null);
      } else {
        setConnectedProviderId(null);
      }
    });
  }, [studioId]);

  const selectedProvider = activeProviders.find((p) => p.id === selectedProviderId);
  const connectedProvider = activeProviders.find((p) => p.id === connectedProviderId);

  const handleConnect = () => {
    if (!selectedProvider) return;
    setError(null);
    setSuccess(null);
    const missing = selectedProvider.authFieldDefs.filter((f) => !credentials[f.key]?.trim());
    if (missing.length > 0) {
      setError(`Fill in: ${missing.map((f) => f.label || f.key).join(', ')}`);
      return;
    }
    startTransition(async () => {
      const result = await connectStudioToProvider(studioId, selectedProvider.id, credentials);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      setConnectedProviderId(selectedProvider.id);
      setCredentials({});
      setSuccess(`Connected to ${selectedProvider.name}.`);
    });
  };

  const handleDisconnect = () => {
    if (!connectedProvider) return;
    setError(null);
    setSuccess(null);
    startTransition(async () => {
      const result = await disconnectStudio(studioId, connectedProvider.id);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      setConnectedProviderId(null);
      setSuccess('Disconnected.');
    });
  };

  if (activeProviders.length === 0) {
    return (
      <Card title={<span className="flex items-center gap-2"><Building2 className="h-4 w-4 text-brand-500" />Studio connections</span>}>
        <EmptyState
          icon={<Link2 className="h-6 w-6" />}
          title="No active CRMs yet"
          description="Add and activate a CRM above before connecting a studio to it."
        />
      </Card>
    );
  }

  return (
    <Card
      title={<span className="flex items-center gap-2"><Building2 className="h-4 w-4 text-brand-500" />Studio connections</span>}
      subtitle="Pick a studio, pick which CRM it uses, and enter its own account credentials."
    >
      <div className="space-y-4">
        <div>
          <Label>Studio</Label>
          <Select value={studioId} onChange={(e) => setStudioId(e.target.value)}>
            {studios.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </Select>
        </div>

        {connectedProviderId === undefined ? (
          <p className="text-sm text-zinc-500">Checking current connection…</p>
        ) : connectedProvider ? (
          <div className="flex items-center justify-between rounded-lg border border-emerald-200 bg-emerald-50/60 p-4 dark:border-emerald-900/40 dark:bg-emerald-950/20">
            <div className="flex items-center gap-2">
              <Badge tone="success">Connected</Badge>
              <span className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">{connectedProvider.name}</span>
            </div>
            <Button size="sm" variant="outline" leftIcon={<Unlink className="h-3.5 w-3.5" />} loading={pending} onClick={handleDisconnect}>
              Disconnect
            </Button>
          </div>
        ) : (
          <>
            <div>
              <Label>CRM</Label>
              <Select value={selectedProviderId} onChange={(e) => { setSelectedProviderId(e.target.value); setCredentials({}); }}>
                {activeProviders.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </Select>
            </div>
            {selectedProvider && selectedProvider.authFieldDefs.length > 0 && (
              <div className="space-y-3 rounded-lg border border-zinc-200 p-4 dark:border-zinc-800">
                {selectedProvider.authFieldDefs.map((f) => (
                  <div key={f.key}>
                    <Label>{f.label || f.key}</Label>
                    <Input
                      type={f.secret ? 'password' : 'text'}
                      value={credentials[f.key] ?? ''}
                      onChange={(e) => setCredentials((c) => ({ ...c, [f.key]: e.target.value }))}
                    />
                  </div>
                ))}
              </div>
            )}
            <Button leftIcon={<Link2 className="h-4 w-4" />} loading={pending} onClick={handleConnect}>
              Connect
            </Button>
          </>
        )}

        {error && <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>}
        {success && <p className="flex items-center gap-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400"><Check className="h-4 w-4" />{success}</p>}
      </div>
    </Card>
  );
}
