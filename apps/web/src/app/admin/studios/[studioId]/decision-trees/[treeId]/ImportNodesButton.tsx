'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { Upload, X, FileText, CheckCircle2, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { ApiError } from '@/lib/api';

interface ImportRowError {
  rowNum: number;
  label?: string;
  error: string;
}

interface ImportResult {
  created: number;
  errors: ImportRowError[];
}

export function ImportNodesButton({
  studioId,
  treeId,
  onImported,
}: {
  studioId: string;
  treeId: string;
  onImported: () => void | Promise<void>;
}) {
  const [mounted, setMounted] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [dragActive, setDragActive] = useState(false);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  function reset() {
    setFile(null);
    setResult(null);
    setError(null);
  }

  function handleDrag(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') setDragActive(true);
    else if (e.type === 'dragleave') setDragActive(false);
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const droppedFile = e.dataTransfer.files[0];
      const ext = droppedFile.name.split('.').pop()?.toLowerCase();
      if (ext === 'xlsx' || ext === 'xls') setFile(droppedFile);
    }
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    if (e.target.files && e.target.files[0]) setFile(e.target.files[0]);
  }

  async function handleImport(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setLoading(true);
    setError(null);
    setResult(null);

    const formData = new FormData();
    formData.append('file', file);

    try {
      const res = await fetch(
        `/api/v1/studios/${studioId}/decision-trees/${treeId}/nodes/import`,
        { method: 'POST', body: formData, credentials: 'include' },
      );
      const body = await res.json().catch(() => null);
      if (!res.ok) {
        throw new ApiError(body?.error ?? `HTTP ${res.status}`, res.status, body?.code, body?.details);
      }
      const data = body as ImportResult;
      setResult({ ...data, errors: data.errors ?? [] });
      if (data.created > 0) {
        await onImported();
      }
    } catch (err: any) {
      setError(err.message || 'Failed to import nodes.');
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        leftIcon={<Upload className="h-3.5 w-3.5" />}
        onClick={() => setIsOpen(true)}
      >
        Import
      </Button>

      {isOpen && mounted && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div
            className="w-full max-w-lg overflow-hidden rounded-[28px] border border-white/30 bg-white/80 p-6 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-zinc-900/80"
            style={{ boxShadow: '0 20px 50px rgba(0,0,0,0.3)' }}
          >
            <div className="flex items-center justify-between border-b border-zinc-200/50 pb-4 dark:border-zinc-800/50">
              <h2 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Import Decision Tree Nodes</h2>
              <button
                onClick={() => { setIsOpen(false); reset(); }}
                className="rounded-full p-1 text-zinc-400 hover:bg-zinc-200/50 dark:hover:bg-zinc-800/50"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleImport} className="mt-5 space-y-5">
              <div className="flex items-center justify-between rounded-2xl border border-zinc-200/50 bg-zinc-100/50 p-4 dark:border-zinc-800/50 dark:bg-zinc-800/30">
                <div className="flex flex-col min-w-0 pr-2">
                  <span className="text-xs font-black text-zinc-900 dark:text-white">Import Template</span>
                  <span className="text-[10px] text-zinc-400 truncate">
                    Download the format for Label, Parent Label, Condition, Reply, and Action columns
                  </span>
                </div>
                <a
                  href={`/api/v1/studios/${studioId}/decision-trees/import-template`}
                  download="decision-tree-template.xlsx"
                  className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-violet-500/10 px-3.5 py-1.5 text-[10px] font-black uppercase tracking-widest text-violet-600 dark:text-violet-400 hover:bg-violet-500/20 transition-colors"
                >
                  Download
                </a>
              </div>

              <div>
                <div
                  onDragEnter={handleDrag}
                  onDragOver={handleDrag}
                  onDragLeave={handleDrag}
                  onDrop={handleDrop}
                  className={`flex flex-col items-center justify-center rounded-[20px] border-2 border-dashed p-8 transition-all ${
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
                      <Upload className="h-10 w-10 text-zinc-400" />
                      <span className="mt-2 text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                        Drag and drop your file here, or <span className="text-brand-500 hover:underline">browse</span>
                      </span>
                      <span className="mt-1 text-xs text-zinc-400">Supports XLSX or XLS files</span>
                      <input type="file" accept=".xlsx,.xls" onChange={handleFileChange} className="hidden" />
                    </label>
                  )}
                </div>
              </div>

              {error && (
                <div className="flex items-start gap-3 rounded-2xl bg-rose-500/10 p-4 text-sm font-medium text-rose-600 dark:text-rose-400">
                  <AlertCircle className="h-5 w-5 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              {result && (
                <div className="space-y-2">
                  <div
                    className={`flex items-start gap-3 rounded-2xl p-4 text-sm font-medium ${
                      result.created > 0
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'bg-amber-500/10 text-amber-600 dark:text-amber-400'
                    }`}
                  >
                    {result.created > 0 ? <CheckCircle2 className="h-5 w-5 shrink-0" /> : <AlertCircle className="h-5 w-5 shrink-0" />}
                    <span>
                      {result.created} node{result.created === 1 ? '' : 's'} created.
                      {result.errors.length > 0 && ` ${result.errors.length} row${result.errors.length === 1 ? '' : 's'} skipped.`}
                    </span>
                  </div>
                  {result.errors.length > 0 && (
                    <div className="max-h-40 overflow-y-auto rounded-2xl border border-zinc-200/50 dark:border-zinc-800/50">
                      {result.errors.map((e, i) => (
                        <div
                          key={i}
                          className="border-b border-zinc-200/30 px-3 py-2 text-xs last:border-b-0 dark:border-zinc-800/30"
                        >
                          <span className="font-bold text-zinc-700 dark:text-zinc-300">Row {e.rowNum}</span>
                          {e.label && <span className="text-zinc-400"> ({e.label})</span>}
                          <span className="text-rose-500"> — {e.error}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              <div className="flex items-center justify-end gap-3 border-t border-zinc-200/50 pt-4 dark:border-zinc-800/50">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => { setIsOpen(false); reset(); }}
                  disabled={loading}
                >
                  {result ? 'Close' : 'Cancel'}
                </Button>
                <Button type="submit" loading={loading} disabled={!file}>
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
