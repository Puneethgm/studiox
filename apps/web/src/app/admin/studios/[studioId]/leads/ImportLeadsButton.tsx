'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useRouter } from 'next/navigation';
import { Upload, X, FileText, CheckCircle2, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Label } from '@/components/ui/Label';
import type { Campaign } from '@/lib/types';
import { importLeadsAction } from './actions';

interface ImportLeadsButtonProps {
  studioId: string;
  campaigns: Campaign[];
}

export function ImportLeadsButton({ studioId, campaigns }: ImportLeadsButtonProps) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [campaignId, setCampaignId] = useState(campaigns[0]?.id || '');
  const [file, setFile] = useState<File | null>(null);
  const [dragActive, setDragActive] = useState(false);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  function handleDrag(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      setDragActive(true);
    } else if (e.type === "dragleave") {
      setDragActive(false);
    }
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const droppedFile = e.dataTransfer.files[0];
      const ext = droppedFile.name.split('.').pop()?.toLowerCase();
      if (ext === 'csv' || ext === 'xlsx' || ext === 'xls') {
        setFile(droppedFile);
      }
    }
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    if (e.target.files && e.target.files[0]) {
      setFile(e.target.files[0]);
    }
  }

  async function handleImport(e: React.FormEvent) {
    e.preventDefault();
    if (!file || !campaignId) return;

    setLoading(true);
    setResult(null);

    const formData = new FormData();
    formData.append('file', file);
    formData.append('campaignId', campaignId);

    try {
      const res = await importLeadsAction(studioId, formData);
      if (res.ok) {
        setResult({ ok: true, message: res.message || 'Leads imported successfully.' });
        router.refresh();
        setTimeout(() => {
          setIsOpen(false);
          setFile(null);
          setResult(null);
        }, 2000);
      } else {
        setResult({ ok: false, message: res.error || 'Failed to import leads.' });
      }
    } catch (err: any) {
      setResult({ ok: false, message: err.message || 'An error occurred during import.' });
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <Button
        onClick={() => setIsOpen(true)}
        variant="ghost"
        className="flex items-center gap-2 rounded-full border border-white/20 bg-white/10 px-4 py-2 text-xs font-bold text-zinc-700 hover:bg-white/20 dark:text-zinc-200 dark:hover:bg-neutral-800/50"
      >
        <Upload className="h-4 w-4" />
        Import Leads
      </Button>

      {isOpen && mounted && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div
            className="w-full max-w-lg overflow-hidden rounded-[28px] border border-white/30 bg-white/80 p-6 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-zinc-900/80"
            style={{ boxShadow: '0 20px 50px rgba(0,0,0,0.3)' }}
          >
            <div className="flex items-center justify-between border-b border-zinc-200/50 pb-4 dark:border-zinc-800/50">
              <h2 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Import Leads</h2>
              <button
                onClick={() => {
                  setIsOpen(false);
                  setFile(null);
                  setResult(null);
                }}
                className="rounded-full p-1 text-zinc-400 hover:bg-zinc-200/50 dark:hover:bg-zinc-800/50"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleImport} className="mt-5 space-y-5">
              <div>
                <Label htmlFor="campaign-select">Default Campaign</Label>
                <select
                  id="campaign-select"
                  value={campaignId}
                  onChange={(e) => setCampaignId(e.target.value)}
                  className="mt-1.5 w-full rounded-2xl border border-zinc-200 bg-white/50 px-4 py-3 text-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800 dark:bg-zinc-950/50"
                >
                  {campaigns.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
                <p className="mt-1 text-[11px] text-zinc-400">
                  Leads will be assigned to this campaign if not specified in the file.
                </p>
              </div>

              <div>
                <Label>File Upload (CSV, XLSX)</Label>
                <div
                  onDragEnter={handleDrag}
                  onDragOver={handleDrag}
                  onDragLeave={handleDrag}
                  onDrop={handleDrop}
                  className={`mt-2 flex flex-col items-center justify-center rounded-[20px] border-2 border-dashed p-8 transition-all ${
                    dragActive
                      ? 'border-brand-500 bg-brand-500/5'
                      : file
                      ? 'border-emerald-500 bg-emerald-500/5'
                      : 'border-zinc-300 dark:border-zinc-700 bg-transparent hover:bg-zinc-500/5'
                  }`}
                >
                  {file ? (
                    <div className="flex flex-col items-center text-center">
                      <FileText className="h-10 w-10 text-emerald-500" />
                      <span className="mt-2 text-sm font-bold text-zinc-800 dark:text-zinc-200">{file.name}</span>
                      <span className="text-xs text-zinc-400">{(file.size / 1024).toFixed(1)} KB</span>
                      <button
                        type="button"
                        onClick={() => setFile(null)}
                        className="mt-3 text-xs font-semibold text-rose-500 hover:underline"
                      >
                        Remove file
                      </button>
                    </div>
                  ) : (
                    <label className="flex flex-col items-center cursor-pointer text-center">
                      <Upload className="h-10 w-10 text-zinc-400 group-hover:scale-110 transition-transform" />
                      <span className="mt-2 text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                        Drag and drop your file here, or <span className="text-brand-500 hover:underline">browse</span>
                      </span>
                      <span className="mt-1 text-xs text-zinc-400">Supports CSV, XLSX or XLS files</span>
                      <input
                        type="file"
                        accept=".csv,.xlsx,.xls"
                        onChange={handleFileChange}
                        className="hidden"
                      />
                    </label>
                  )}
                </div>
              </div>

              {result && (
                <div
                  className={`flex items-start gap-3 rounded-2xl p-4 text-sm font-medium ${
                    result.ok
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                  }`}
                >
                  {result.ok ? <CheckCircle2 className="h-5 w-5 shrink-0" /> : <AlertCircle className="h-5 w-5 shrink-0" />}
                  <span>{result.message}</span>
                </div>
              )}

              <div className="flex items-center justify-end gap-3 border-t border-zinc-200/50 pt-4 dark:border-zinc-800/50">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    setIsOpen(false);
                    setFile(null);
                    setResult(null);
                  }}
                  disabled={loading}
                >
                  Cancel
                </Button>
                <Button type="submit" loading={loading} disabled={!file || !campaignId}>
                  Start Import
                </Button>
              </div>
            </form>
          </div>
        </div>,
        document.body
      )}
    </>
  );
}
