'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Database, FileText, Trash2, Upload, AlertCircle, CheckCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Label, FieldHint } from '@/components/ui/Label';
import type { Studio } from '@/lib/types';
import { api } from '@/lib/api';
import { parseDocument, updateKnowledgeBase } from './actions';

export function KnowledgeBaseForm({ studio }: { studio: Studio }) {
  const router = useRouter();
  const [text, setText] = useState(studio.knowledgeBase || '');
  const [files, setFiles] = useState<{ name: string; url: string; text: string }[]>(
    studio.knowledgeBaseFiles || []
  );
  
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

  async function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const filesList = e.target.files;
    if (!filesList || filesList.length === 0) return;

    setError(null);
    setSuccess(null);
    setUploading(true);

    try {
      const uploadPromises = Array.from(filesList).map(async (file) => {
        // 1. Upload to Go Backend uploads directory
        const formData = new FormData();
        formData.append('file', file);

        const uploadRes = await fetch(`/api/v1/studios/${studio.id}/messaging/upload`, {
          method: 'POST',
          body: formData,
        });

        if (!uploadRes.ok) {
          const errText = await uploadRes.text().catch(() => '');
          throw new Error(errText || `Upload failed with status ${uploadRes.status}`);
        }

        const uploadResult = await uploadRes.json() as { url: string };

        // 2. Parse text from document using server action
        const parseResult = await parseDocument(formData);
        const data = parseResult.data;
        if (!parseResult.ok || !data) {
          throw new Error(parseResult.error || `Failed to extract text from "${file.name}"`);
        }

        return {
          name: file.name,
          url: uploadResult.url,
          text: data.text,
        };
      });

      const parsedFiles = await Promise.all(uploadPromises);
      setFiles((prev) => [...prev, ...parsedFiles]);
      setSuccess(`Successfully uploaded and processed ${parsedFiles.length} file(s)!`);
    } catch (err: any) {
      console.error(err);
      setError(err.message || 'Failed to upload and parse files.');
    } finally {
      setUploading(false);
      // Clear the input value so the same files can be uploaded again
      e.target.value = '';
    }
  }

  function handleRemoveFile(index: number) {
    setError(null);
    setSuccess(null);
    setFiles((prev) => prev.filter((_, i) => i !== index));
  }

  async function handleSave() {
    setError(null);
    setSuccess(null);
    setSaving(true);

    try {
      const res = await updateKnowledgeBase(studio.id, studio.slug, text, files);
      if (!res.ok) {
        throw new Error(res.error || 'Failed to save changes');
      }
      setSuccess('Knowledge base updated successfully!');
      router.refresh();
    } catch (err: any) {
      setError(err.message || 'An error occurred while saving.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Messages */}
      {success && (
        <div className="flex items-center gap-3 rounded-2xl border border-emerald-500/20 bg-emerald-500/5 p-4 text-sm font-semibold text-emerald-600 dark:text-emerald-400">
          <CheckCircle className="h-5 w-5 shrink-0" />
          <span>{success}</span>
        </div>
      )}
      {error && (
        <div className="flex items-center gap-3 rounded-2xl border border-rose-500/20 bg-rose-500/5 p-4 text-sm font-semibold text-rose-600 dark:text-rose-400">
          <AlertCircle className="h-5 w-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <div className="grid gap-6 md:grid-cols-3">
        {/* Left column: Text Instructions & File List */}
        <div className="md:col-span-2 space-y-6">
          <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
            <div className="border-b border-white/20 px-6 py-4 dark:border-white/5 flex items-center gap-2">
              <Database className="h-4 w-4 text-zinc-400" />
              <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Text Instructions</h3>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <Label htmlFor="knowledgeBase">Instructions / General Info</Label>
                <textarea
                  id="knowledgeBase"
                  className="flex w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm placeholder:text-slate-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:placeholder:text-slate-500 min-h-[250px]"
                  placeholder="Paste FAQs, price details, general studio guidelines here. The AI will read this to answer customer questions..."
                  value={text}
                  onChange={(e) => setText(e.target.value)}
                />
                <FieldHint>Direct textual instructions for the AI assistant.</FieldHint>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
            <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
              <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Uploaded Documents</h3>
            </div>
            <div className="p-6">
              {files.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-10 text-center">
                  <FileText className="h-10 w-10 text-zinc-400 dark:text-zinc-600 mb-2" />
                  <p className="text-sm font-semibold text-zinc-500 dark:text-zinc-400">No documents uploaded yet</p>
                  <p className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">Upload files to feed details directly into the AI.</p>
                </div>
              ) : (
                <div className="divide-y divide-white/10 dark:divide-white/5">
                  {files.map((file, i) => (
                    <div key={i} className="flex items-center justify-between py-3">
                      <div className="flex items-center gap-3 min-w-0">
                        <FileText className="h-5 w-5 text-brand-500 shrink-0" />
                        <div className="min-w-0">
                          {file.url ? (
                            <a
                              href={file.url}
                              target="_blank"
                              rel="noreferrer"
                              className="text-sm font-semibold hover:underline text-zinc-800 dark:text-white truncate block"
                            >
                              {file.name}
                            </a>
                          ) : (
                            <span className="text-sm font-semibold text-zinc-800 dark:text-white truncate block">
                              {file.name}
                            </span>
                          )}
                          <p className="text-xs text-zinc-400 dark:text-zinc-500">
                            {file.text ? `${file.text.substring(0, 100)}...` : 'Processing...'}
                          </p>
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => handleRemoveFile(i)}
                        className="rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 hover:text-rose-500 dark:hover:bg-zinc-800/50"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Right column: Upload Area & Controls */}
        <div className="space-y-6">
          <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
            <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
              <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Add Documents</h3>
            </div>
            <div className="p-6 space-y-4">
              <div className="relative flex flex-col items-center justify-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-2xl p-6 text-center hover:bg-zinc-50/50 dark:hover:bg-zinc-950/20 transition cursor-pointer">
                <input
                  type="file"
                  id="file-upload"
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer disabled:cursor-not-allowed"
                  disabled={uploading}
                  onChange={handleFileUpload}
                  multiple
                  accept=".pdf,.docx,.doc,.pptx,.ppt,.xlsx,.xls,.txt,.csv,.md"
                />
                <Upload className="h-8 w-8 text-zinc-400 mb-2" />
                <span className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                  {uploading ? 'Processing file...' : 'Choose a document'}
                </span>
                <span className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">
                  PDF, Word, PowerPoint, Excel, Text, CSV
                </span>
              </div>
              <FieldHint>
                Uploaded files are parsed by the AI helper to build dynamic context when conversing with leads.
              </FieldHint>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Button
              onClick={handleSave}
              loading={saving}
              className="w-full text-center py-3"
            >
              Save Knowledge Base
            </Button>
            <Button
              variant="ghost"
              onClick={() => router.back()}
              className="w-full text-center"
            >
              Cancel
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
