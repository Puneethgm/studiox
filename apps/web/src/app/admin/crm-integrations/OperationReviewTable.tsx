'use client';

import { useState, useTransition } from 'react';
import { ChevronDown, ChevronRight, Check, Plus, Trash2, Rocket, X } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Textarea } from '@/components/ui/Textarea';
import { Label, FieldHint } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { activateProvider, updateOperation, updateProviderDetails } from './actions';
import { OPERATION_CATALOG, type AuthFieldDef, type AuthType, type CrmOperation, type CrmProvider, type OperationKey } from './types';

export function OperationReviewTable({
  provider,
  operations,
  onClose,
  onChanged,
}: {
  provider: CrmProvider;
  operations: CrmOperation[];
  onClose: () => void;
  onChanged: (provider: CrmProvider) => void;
}) {
  const [currentProvider, setCurrentProvider] = useState(provider);
  const [opsByKey, setOpsByKey] = useState<Record<string, CrmOperation | undefined>>(
    Object.fromEntries(operations.map((o) => [o.operationKey, o])),
  );

  return (
    <Card
      title={
        <span className="flex items-center gap-2">
          {currentProvider.name}
          <Badge tone={currentProvider.status === 'active' ? 'success' : 'warning'}>{currentProvider.status}</Badge>
        </span>
      }
      subtitle="Review what the AI found. Fix anything wrong, then activate so studios can connect to it."
      action={
        <Button size="sm" variant="ghost" leftIcon={<X className="h-3.5 w-3.5" />} onClick={onClose}>
          Close
        </Button>
      }
    >
      <div className="space-y-6">
        <AuthSection
          provider={currentProvider}
          onSaved={(p) => { setCurrentProvider(p); onChanged(p); }}
        />

        <div>
          <h4 className="mb-2 text-xs font-black uppercase tracking-wider text-zinc-500">Operations</h4>
          <div className="space-y-2">
            {OPERATION_CATALOG.map((spec) => (
              <OperationRow
                key={spec.key}
                providerId={currentProvider.id}
                spec={spec}
                operation={opsByKey[spec.key]}
                onSaved={(op) => setOpsByKey((prev) => ({ ...prev, [spec.key]: op }))}
              />
            ))}
          </div>
        </div>

        {currentProvider.status === 'draft' && (
          <ActivateButton providerId={currentProvider.id} onActivated={() => onChanged({ ...currentProvider, status: 'active' })} />
        )}
      </div>
    </Card>
  );
}

function AuthSection({ provider, onSaved }: { provider: CrmProvider; onSaved: (p: CrmProvider) => void }) {
  const [editing, setEditing] = useState(false);
  const [description, setDescription] = useState(provider.description);
  const [baseUrl, setBaseUrl] = useState(provider.baseUrl);
  const [authType, setAuthType] = useState<AuthType>(provider.authType);
  const [fields, setFields] = useState<AuthFieldDef[]>(provider.authFieldDefs ?? []);
  const [tokenLoginPath, setTokenLoginPath] = useState(provider.tokenLoginPath ?? '');
  const [tokenLoginMethod, setTokenLoginMethod] = useState(provider.tokenLoginMethod || 'POST');
  const [tokenLoginBody, setTokenLoginBody] = useState<{ bodyField: string; source: string }[]>(
    Object.entries(provider.tokenLoginBodyMapping ?? {}).map(([bodyField, source]) => ({ bodyField, source })),
  );
  const [tokenResponsePath, setTokenResponsePath] = useState(provider.tokenResponsePath ?? '');
  const [tokenExpiryPath, setTokenExpiryPath] = useState(provider.tokenExpiryPath ?? '');
  const [tokenExpirySeconds, setTokenExpirySeconds] = useState(provider.tokenExpirySeconds ?? 0);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const addField = () => setFields((f) => [...f, { key: '', label: '', secret: true }]);
  const removeField = (idx: number) => setFields((f) => f.filter((_, i) => i !== idx));
  const updateField = (idx: number, patch: Partial<AuthFieldDef>) =>
    setFields((f) => f.map((field, i) => (i === idx ? { ...field, ...patch } : field)));

  const addLoginBodyRow = () => setTokenLoginBody((rows) => [...rows, { bodyField: '', source: fields[0]?.key ?? 'const:' }]);
  const removeLoginBodyRow = (idx: number) => setTokenLoginBody((rows) => rows.filter((_, i) => i !== idx));
  const updateLoginBodyRow = (idx: number, patch: Partial<{ bodyField: string; source: string }>) =>
    setTokenLoginBody((rows) => rows.map((row, i) => (i === idx ? { ...row, ...patch } : row)));

  const save = () => {
    setError(null);
    startTransition(async () => {
      const result = await updateProviderDetails(provider.id, {
        description,
        baseUrl,
        authType,
        authFieldDefs: fields,
        ...(authType === 'token_exchange'
          ? {
              tokenLoginPath,
              tokenLoginMethod,
              tokenLoginBodyMapping: Object.fromEntries(tokenLoginBody.filter((r) => r.bodyField).map((r) => [r.bodyField, r.source])),
              tokenResponsePath,
              tokenExpiryPath,
              tokenExpirySeconds,
            }
          : {}),
      });
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onSaved(result.data);
      setEditing(false);
    });
  };

  if (!editing) {
    return (
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-lg border border-zinc-200 bg-zinc-50/50 p-4 text-sm dark:border-zinc-800 dark:bg-zinc-900/40">
        <div><span className="font-black uppercase tracking-wider text-[10px] text-zinc-500">Base URL</span><div className="font-mono text-xs">{provider.baseUrl || '—'}</div></div>
        <div><span className="font-black uppercase tracking-wider text-[10px] text-zinc-500">Auth type</span><div>{AUTH_LABELS[provider.authType]}</div></div>
        <div><span className="font-black uppercase tracking-wider text-[10px] text-zinc-500">Credential fields</span><div>{provider.authFieldDefs?.map((f) => f.label || f.key).join(', ') || '—'}</div></div>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEditing(true)}>Edit</Button>
      </div>
    );
  }

  return (
    <div className="space-y-4 rounded-lg border border-zinc-200 p-4 dark:border-zinc-800">
      <div>
        <Label>Description</Label>
        <Input value={description} onChange={(e) => setDescription(e.target.value)} />
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div>
          <Label>Base URL</Label>
          <Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.example.com" />
        </div>
        <div>
          <Label>How this CRM authenticates</Label>
          <Select value={authType} onChange={(e) => setAuthType(e.target.value as AuthType)}>
            <option value="bearer">Bearer token</option>
            <option value="api_key">API key header(s)</option>
            <option value="basic">Username &amp; password</option>
            <option value="token_exchange">API key header(s) + a login step for a bearer token</option>
          </Select>
        </div>
      </div>
      <div>
        <Label>Credential fields a studio will need to enter</Label>
        <div className="space-y-2">
          {fields.map((f, idx) => (
            <div key={idx} className="flex items-center gap-2">
              <Input className="w-40" placeholder="key (e.g. api_key)" value={f.key} onChange={(e) => updateField(idx, { key: e.target.value })} />
              <Input className="flex-1" placeholder="Label shown to the admin" value={f.label} onChange={(e) => updateField(idx, { label: e.target.value })} />
              <label className="flex shrink-0 items-center gap-1.5 text-xs font-medium text-zinc-600 dark:text-zinc-400">
                <input type="checkbox" checked={f.secret} onChange={(e) => updateField(idx, { secret: e.target.checked })} />
                Secret
              </label>
              <Button size="sm" variant="ghost" onClick={() => removeField(idx)}><Trash2 className="h-3.5 w-3.5" /></Button>
            </div>
          ))}
          <Button size="sm" variant="outline" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={addField}>Add field</Button>
        </div>
        <FieldHint>e.g. Glofox needs three fields: API Key, API Token, Branch ID. A simple bearer-token CRM needs just one.</FieldHint>
      </div>

      {authType === 'token_exchange' && (
        <div className="space-y-4 rounded-lg border border-dashed border-zinc-300 p-4 dark:border-zinc-700">
          <p className="text-xs font-medium text-zinc-500">
            This CRM also needs a login step: it trades the credentials above for a short-lived bearer token, which gets refreshed automatically once it expires.
          </p>
          <div className="grid grid-cols-[100px_1fr] gap-3">
            <div>
              <Label>Method</Label>
              <Select value={tokenLoginMethod} onChange={(e) => setTokenLoginMethod(e.target.value)}>
                <option value="POST">POST</option>
                <option value="GET">GET</option>
              </Select>
            </div>
            <div>
              <Label>Login path</Label>
              <Input className="font-mono text-xs" value={tokenLoginPath} onChange={(e) => setTokenLoginPath(e.target.value)} placeholder="/usertoken/issue" />
            </div>
          </div>

          <div>
            <Label>Login request body</Label>
            <div className="space-y-2">
              {tokenLoginBody.map((row, idx) => (
                <div key={idx} className="flex items-center gap-2">
                  <Input className="w-40" placeholder="Field name (e.g. Username)" value={row.bodyField} onChange={(e) => updateLoginBodyRow(idx, { bodyField: e.target.value })} />
                  <Select
                    className="flex-1"
                    value={row.source.startsWith('const:') ? 'const:' : row.source}
                    onChange={(e) => updateLoginBodyRow(idx, { source: e.target.value === 'const:' ? 'const:' : e.target.value })}
                  >
                    {fields.map((f) => (
                      <option key={f.key} value={f.key}>Use {f.label || f.key}</option>
                    ))}
                    <option value="const:">Fixed value...</option>
                  </Select>
                  {row.source.startsWith('const:') && (
                    <Input
                      className="w-40"
                      placeholder="Fixed value"
                      value={row.source.slice('const:'.length)}
                      onChange={(e) => updateLoginBodyRow(idx, { source: 'const:' + e.target.value })}
                    />
                  )}
                  <Button size="sm" variant="ghost" onClick={() => removeLoginBodyRow(idx)}><Trash2 className="h-3.5 w-3.5" /></Button>
                </div>
              ))}
              <Button size="sm" variant="outline" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={addLoginBodyRow}>Add field</Button>
            </div>
            <FieldHint>e.g. Mindbody's login needs Username = the fixed value "Siteowner" and Password = the studio's API key.</FieldHint>
          </div>

          <div className="grid gap-4 sm:grid-cols-3">
            <div>
              <Label>Token field in response</Label>
              <Input className="font-mono text-xs" value={tokenResponsePath} onChange={(e) => setTokenResponsePath(e.target.value)} placeholder="AccessToken" />
            </div>
            <div>
              <Label>Expiry field in response (optional)</Label>
              <Input className="font-mono text-xs" value={tokenExpiryPath} onChange={(e) => setTokenExpiryPath(e.target.value)} placeholder="ExpiresIn" />
            </div>
            <div>
              <Label>Or a fixed lifetime, in seconds</Label>
              <Input type="number" value={tokenExpirySeconds || ''} onChange={(e) => setTokenExpirySeconds(Number(e.target.value) || 0)} placeholder="3300" />
            </div>
          </div>
        </div>
      )}

      {error && <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>}
      <div className="flex gap-2">
        <Button size="sm" leftIcon={<Check className="h-3.5 w-3.5" />} loading={pending} onClick={save}>Save</Button>
        <Button size="sm" variant="ghost" disabled={pending} onClick={() => setEditing(false)}>Cancel</Button>
      </div>
    </div>
  );
}

const AUTH_LABELS: Record<AuthType, string> = {
  bearer: 'Bearer token',
  api_key: 'API key header(s)',
  basic: 'Username & password',
  token_exchange: 'API key header(s) + login for a bearer token',
};

function OperationRow({
  providerId,
  spec,
  operation,
  onSaved,
}: {
  providerId: string;
  spec: { key: OperationKey; label: string; description: string };
  operation: CrmOperation | undefined;
  onSaved: (op: CrmOperation) => void;
}) {
  const [open, setOpen] = useState(false);
  const [httpMethod, setHttpMethod] = useState(operation?.httpMethod ?? 'GET');
  const [pathTemplate, setPathTemplate] = useState(operation?.pathTemplate ?? '');
  const [requestMapping, setRequestMapping] = useState(JSON.stringify(operation?.requestMapping ?? {}, null, 2));
  const [responseMapping, setResponseMapping] = useState(JSON.stringify(operation?.responseMapping ?? {}, null, 2));
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const found = Boolean(operation);

  const save = () => {
    setError(null);
    let reqObj: Record<string, unknown>;
    let respObj: Record<string, unknown>;
    try {
      reqObj = JSON.parse(requestMapping || '{}');
      respObj = JSON.parse(responseMapping || '{}');
    } catch {
      setError('Request/response mapping must be valid JSON.');
      return;
    }
    if (!pathTemplate.trim()) {
      setError('Path is required, e.g. /v2/members/{id}');
      return;
    }
    startTransition(async () => {
      const result = await updateOperation(providerId, {
        operationKey: spec.key,
        httpMethod,
        pathTemplate,
        requestMapping: reqObj,
        responseMapping: respObj,
        reviewed: true,
      });
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onSaved(result.data);
      setOpen(false);
    });
  };

  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-800">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
      >
        <div className="flex items-center gap-3">
          {open ? <ChevronDown className="h-4 w-4 text-zinc-400" /> : <ChevronRight className="h-4 w-4 text-zinc-400" />}
          <div>
            <div className="text-sm font-bold text-zinc-800 dark:text-zinc-100">{spec.label}</div>
            <div className="text-xs text-zinc-500">{spec.description}</div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {found ? (
            <>
              <span className="font-mono text-xs text-zinc-500">{operation!.httpMethod} {operation!.pathTemplate}</span>
              <Badge tone={operation!.reviewed ? 'success' : 'warning'}>{operation!.reviewed ? 'Reviewed' : 'Needs review'}</Badge>
            </>
          ) : (
            <Badge tone="neutral">Not found</Badge>
          )}
        </div>
      </button>
      {open && (
        <div className="space-y-3 border-t border-zinc-200 p-4 dark:border-zinc-800">
          <div className="grid grid-cols-[100px_1fr] gap-3">
            <div>
              <Label>Method</Label>
              <Select value={httpMethod} onChange={(e) => setHttpMethod(e.target.value)}>
                {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((m) => <option key={m} value={m}>{m}</option>)}
              </Select>
            </div>
            <div>
              <Label>Path</Label>
              <Input className="font-mono text-xs" value={pathTemplate} onChange={(e) => setPathTemplate(e.target.value)} placeholder="/v2/members/{userId}" />
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label>Request mapping (JSON)</Label>
              <Textarea
                className="font-mono text-xs"
                rows={5}
                value={requestMapping}
                onChange={(e) => setRequestMapping(e.target.value)}
              />
              <FieldHint>{'e.g. {"path":{"userId":"userId"},"body":{"email":"email"}}'}</FieldHint>
            </div>
            <div>
              <Label>Response mapping (JSON)</Label>
              <Textarea
                className="font-mono text-xs"
                rows={5}
                value={responseMapping}
                onChange={(e) => setResponseMapping(e.target.value)}
              />
              <FieldHint>{'e.g. {"userId":"_id","email":"contact.email"}'}</FieldHint>
            </div>
          </div>
          {error && <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>}
          <Button size="sm" leftIcon={<Check className="h-3.5 w-3.5" />} loading={pending} onClick={save}>
            Save &amp; mark reviewed
          </Button>
        </div>
      )}
    </div>
  );
}

function ActivateButton({ providerId, onActivated }: { providerId: string; onActivated: () => void }) {
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const activate = () => {
    setError(null);
    startTransition(async () => {
      const result = await activateProvider(providerId);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onActivated();
    });
  };

  return (
    <div className="flex flex-col items-start gap-2 border-t border-zinc-200 pt-4 dark:border-zinc-800">
      {error && <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>}
      <Button leftIcon={<Rocket className="h-4 w-4" />} loading={pending} onClick={activate}>
        Activate — make this CRM connectable to studios
      </Button>
    </div>
  );
}
