'use client';

import { useEffect, useState } from 'react';
import { UploadCloud, CheckCircle2, XCircle, RefreshCw, FileKey } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { FieldError } from '@/components/ui/Label';
import { api } from '@/lib/api';

interface CredsStatus {
  configured: boolean;
  clientEmail?: string;
  projectId?: string;
  error?: string;
}

export function CredentialsManager() {
  const [status, setStatus] = useState<CredsStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  async function fetchStatus() {
    try {
      const data = await api<CredsStatus>('/api/v1/admin/google-credentials');
      setStatus(data);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch credentials status');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchStatus();
  }, []);

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploading(true);
    setError(null);
    setSuccessMessage(null);

    const formData = new FormData();
    formData.append('file', file);

    try {
      const res = await fetch('/api/v1/admin/google-credentials', {
        method: 'POST',
        body: formData,
      });

      const body = await res.json().catch(() => null);

      if (!res.ok) {
        throw new Error(body?.error ?? `Upload failed (HTTP ${res.status})`);
      }

      setStatus(body);
      setSuccessMessage('Credentials uploaded and verified successfully!');
    } catch (err: any) {
      setError(err.message || 'Failed to upload credentials file');
    } finally {
      setUploading(false);
    }
  }

  if (loading) {
    return (
      <Card>
        <div className="flex h-32 items-center justify-center gap-3">
          <RefreshCw className="h-5 w-5 animate-spin text-zinc-400" />
          <span className="text-sm font-semibold text-zinc-500">Checking credentials status…</span>
        </div>
      </Card>
    );
  }

  return (
    <div
      className="relative overflow-hidden rounded-[26px] border border-white/30 backdrop-blur-2xl dark:border-white/5"
      style={{
        background: 'linear-gradient(135deg, rgba(255,255,255,0.40) 0%, rgba(244,244,245,0.30) 100%)',
        boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.25), 0 8px 32px rgba(9,9,11,0.05)',
      }}
    >
      <div className="p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex items-start gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-indigo-500 to-brand-600 text-white shadow-lg shadow-indigo-500/20">
              <FileKey className="h-6 w-6" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-zinc-900 dark:text-white">Google Integration Credentials</h2>
              <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400 max-w-xl">
                Upload your Google Cloud service account JSON key file (`google-credentials.json`) here to enable automatic lead synchronization to your master Google Sheets CRM spreadsheet.
              </p>
            </div>
          </div>
          <div className="shrink-0">
            {status?.configured ? (
              <Badge tone="success" className="gap-1 px-3 py-1 font-bold">
                <CheckCircle2 className="h-3.5 w-3.5" /> Configured
              </Badge>
            ) : (
              <Badge tone="warning" className="gap-1 px-3 py-1 font-bold">
                <XCircle className="h-3.5 w-3.5" /> Not Configured
              </Badge>
            )}
          </div>
        </div>

        {status?.configured && (
          <div className="mt-6 rounded-2xl border border-emerald-500/20 bg-emerald-500/5 p-4 dark:border-emerald-500/10">
            <h3 className="text-xs font-black uppercase tracking-wider text-emerald-800 dark:text-emerald-400">Current Key Details</h3>
            <dl className="mt-3 grid gap-x-4 gap-y-2 sm:grid-cols-2 text-sm">
              <div>
                <dt className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Client Email (Service Account)</dt>
                <dd className="mt-0.5 font-semibold text-zinc-700 dark:text-zinc-300 break-all select-all">{status.clientEmail}</dd>
              </div>
              <div>
                <dt className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Project ID</dt>
                <dd className="mt-0.5 font-semibold text-zinc-700 dark:text-zinc-300 select-all">{status.projectId}</dd>
              </div>
            </dl>
          </div>
        )}

        <div className="mt-6 flex flex-col items-center justify-center rounded-2xl border border-dashed border-zinc-300 p-8 dark:border-zinc-800">
          <UploadCloud className="h-10 w-10 text-zinc-400 dark:text-zinc-600" />
          <h3 className="mt-3 text-sm font-bold text-zinc-900 dark:text-white">
            {status?.configured ? 'Upload new credentials file' : 'Upload google-credentials.json'}
          </h3>
          <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400 text-center max-w-sm">
            Must be a valid Google Cloud Service Account JSON file with Google Sheets API access.
          </p>

          <div className="mt-4">
            <input
              type="file"
              id="google-creds-file"
              accept=".json"
              className="hidden"
              onChange={handleFileChange}
              disabled={uploading}
            />
            <label htmlFor="google-creds-file" className="cursor-pointer">
              <span className="inline-flex items-center justify-center font-semibold h-9 px-4 text-xs gap-1.5 rounded-2xl bg-white/60 text-zinc-800 dark:bg-white/10 dark:text-zinc-100 shadow-sm border border-white/20 dark:border-white/5 hover:bg-white/80 dark:hover:bg-white/20 transition-all select-none">
                {uploading ? (
                  <>
                    <RefreshCw className="h-3.5 w-3.5 animate-spin" /> Uploading…
                  </>
                ) : (
                  'Choose JSON File'
                )}
              </span>
            </label>
          </div>
          {successMessage && (
            <div className="mt-4 w-full max-w-sm rounded-2xl border border-emerald-500/20 bg-emerald-500/5 p-3 text-xs font-semibold text-emerald-800 dark:text-emerald-400 flex items-center justify-center gap-2 animate-in fade-in slide-in-from-bottom-2 duration-300">
              <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
              <span>{successMessage}</span>
            </div>
          )}

          {error && (
            <div className="mt-4 w-full max-w-sm rounded-2xl border border-red-500/20 bg-red-500/5 p-3 text-xs font-semibold text-red-800 dark:text-red-400 flex items-center justify-center gap-2 animate-in fade-in slide-in-from-bottom-2 duration-300">
              <XCircle className="h-4 w-4 text-red-600 dark:text-red-400" />
              <span>{error}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
