'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter, usePathname, useSearchParams } from 'next/navigation';
import { Database, FileText, Trash2, Upload, AlertCircle, CheckCircle, ChevronDown, Sparkles, Loader2, MessageCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Label, FieldHint } from '@/components/ui/Label';
import { Dialog, DialogHeader } from '@/components/ui/Dialog';
import { api } from '@/lib/api';
import type { Studio } from '@/lib/types';
import { parseDocument, updateKnowledgeBase, updateCommunicationStyle, updateStyleRefreshInterval, updateProgramStartDate } from './actions';
import { TestChatDrawer } from './TestChatDrawer';

type KBFile = { name: string; url: string; text: string; platform: string };

const DOMAINS = [
  { value: 'all',         label: 'General' },
  { value: 'fitness_gym', label: 'Fitness / Gym' },
  { value: 'yoga',        label: 'Yoga' },
  { value: 'crossfit',    label: 'CrossFit' },
  { value: 'pilates',     label: 'Pilates' },
  { value: 'dance',       label: 'Dance' },
  { value: 'martial_arts',label: 'Martial Arts' },
  { value: 'nutrition',   label: 'Nutrition' },
  { value: 'recovery',    label: 'Recovery' },
];

const DOMAIN_COLORS: Record<string, string> = {
  all:         'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300',
  fitness_gym: 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  yoga:        'bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
  crossfit:    'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400',
  pilates:     'bg-pink-50 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  dance:       'bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-400',
  martial_arts:'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  nutrition:   'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  recovery:    'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
};

function PlatformSelect({
  value,
  onChange,
  className = '',
}: {
  value: string;
  onChange: (v: string) => void;
  className?: string;
}) {
  return (
    <div className={`relative inline-flex items-center ${className}`}>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="appearance-none cursor-pointer rounded-lg border border-slate-200 bg-white pr-7 pl-2.5 py-1 text-xs font-semibold focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-slate-700 dark:bg-slate-900"
      >
        {DOMAINS.map((p) => (
          <option key={p.value} value={p.value}>{p.label}</option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-1.5 h-3 w-3 text-zinc-400" />
    </div>
  );
}

// Replaces an explicit Save button for auto-saving fields — a quiet status
// next to the field instead of a big banner, since it fires on every edit.
function AutosaveIndicator({ status }: { status: 'idle' | 'saving' | 'saved' | 'error' }) {
  if (status === 'idle') return null;
  if (status === 'saving') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-zinc-400">
        <Loader2 className="h-3 w-3 animate-spin" />
        Saving…
      </span>
    );
  }
  if (status === 'saved') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
        <CheckCircle className="h-3 w-3" />
        Saved
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-rose-600 dark:text-rose-400">
      <AlertCircle className="h-3 w-3" />
      Failed to save
    </span>
  );
}

type KBSection = 'instructions' | 'style';
const VALID_SECTIONS: KBSection[] = ['instructions', 'style'];

export function KnowledgeBaseForm({ studio }: { studio: Studio }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const initialSection = (VALID_SECTIONS.includes(searchParams.get('tab') as KBSection)
    ? searchParams.get('tab')
    : 'instructions') as KBSection;
  const [activeSection, _setActiveSection] = useState<KBSection>(initialSection);
  const setActiveSection = (section: KBSection) => {
    _setActiveSection(section);
    const p = new URLSearchParams(searchParams.toString());
    p.set('tab', section);
    router.replace(`${pathname}?${p.toString()}`, { scroll: false });
  };

  const [text, setText] = useState(studio.knowledgeBase || '');
  const [greeting, setGreeting] = useState(studio.greetingMessage || '');
  const [files, setFiles] = useState<KBFile[]>(
    (studio.knowledgeBaseFiles || []).map((f) => ({ ...f, platform: f.platform || 'all' }))
  );

  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Auto-save: Knowledge Base (greeting, text instructions, documents) —
  // debounced so a save fires ~900ms after the admin stops typing/uploading
  // instead of requiring an explicit button click. Skips the very first
  // effect run (mount), which would otherwise "save" the untouched initial
  // values loaded from the server.
  const [kbStatus, setKbStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const [kbStatusError, setKbStatusError] = useState<string | null>(null);
  const kbFirstRun = useRef(true);

  useEffect(() => {
    if (kbFirstRun.current) {
      kbFirstRun.current = false;
      return;
    }
    setKbStatus('saving');
    const timer = setTimeout(async () => {
      try {
        const res = await updateKnowledgeBase(studio.id, studio.slug, text, 'all', files, greeting);
        if (!res.ok) throw new Error(res.error || 'Failed to save changes');
        setKbStatus('saved');
        setKbStatusError(null);
        router.refresh();
      } catch (err: any) {
        setKbStatus('error');
        setKbStatusError(err.message || 'An error occurred while saving.');
      }
    }, 900);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, greeting, files]);

  // "Saved" is a transient confirmation, not a permanent state — fade it
  // back to idle so the indicator doesn't sit there forever.
  useEffect(() => {
    if (kbStatus !== 'saved') return;
    const t = setTimeout(() => setKbStatus('idle'), 2500);
    return () => clearTimeout(t);
  }, [kbStatus]);

  // Saving the knowledge base only persists the text/file list — the actual
  // chunking + embedding happens afterward in a background job on the API
  // (see asyncSyncKnowledgeChunks), which can take anywhere from a couple
  // seconds to a couple minutes for a large document. Poll the sync status
  // after every successful save and pop up a real "it's done, go test it"
  // notice once that background job actually finishes, instead of letting
  // the "Successfully uploaded" message imply it's already searchable.
  const [syncOutcome, setSyncOutcome] = useState<'complete' | 'error' | null>(null);
  const [testChatSignal, setTestChatSignal] = useState(0);

  useEffect(() => {
    if (kbStatus !== 'saved') return;
    let cancelled = false;
    let attempts = 0;
    // Small buffer for clock skew between browser and server — only treat a
    // "complete"/"error" status as belonging to THIS save if its timestamp
    // is at/after this point, so an unrelated field edit (e.g. just the
    // greeting, which doesn't re-trigger a sync) doesn't replay a stale
    // popup from a previous, unrelated sync.
    const baseline = Date.now() - 5000;

    async function poll() {
      if (cancelled) return;
      try {
        const res = await api<{ status: string; updatedAt: string | null }>(
          `/api/v1/studios/${studio.id}/knowledge-base/sync-status`,
        );
        const updatedAtMs = res.updatedAt ? new Date(res.updatedAt).getTime() : 0;
        if (updatedAtMs >= baseline && (res.status === 'complete' || res.status === 'error')) {
          setSyncOutcome(res.status);
          return;
        }
      } catch {
        // Transient network hiccup — keep polling rather than giving up.
      }
      attempts += 1;
      // Cap at ~2 minutes of polling so an edit that never triggers a real
      // sync (or a genuinely stuck one) doesn't poll forever.
      if (attempts < 60 && !cancelled) {
        setTimeout(poll, 2000);
      }
    }

    const t = setTimeout(poll, 1200);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  }, [kbStatus, studio.id]);

  // Program start date — anchors a parsed week-by-week program document
  // (e.g. an "8 Week Challenge" PDF) to a real calendar date, so the AI can
  // compute exactly which week/day today falls on with real date math
  // instead of guessing via semantic search across many near-identical
  // weekly entries. studio.programStartDate comes back as a full ISO
  // timestamp; the date input only wants the "YYYY-MM-DD" portion.
  const [programStartDate, setProgramStartDate] = useState(
    studio.programStartDate ? studio.programStartDate.slice(0, 10) : ''
  );
  const [programDateStatus, setProgramDateStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const [programDateError, setProgramDateError] = useState<string | null>(null);
  const programDateFirstRun = useRef(true);

  useEffect(() => {
    if (programDateFirstRun.current) {
      programDateFirstRun.current = false;
      return;
    }
    setProgramDateStatus('saving');
    const timer = setTimeout(async () => {
      try {
        const res = await updateProgramStartDate(studio.id, programStartDate);
        if (!res.ok) throw new Error(res.error || 'Failed to save changes');
        setProgramDateStatus('saved');
        setProgramDateError(null);
        router.refresh();
      } catch (err: any) {
        setProgramDateStatus('error');
        setProgramDateError(err.message || 'An error occurred while saving.');
      }
    }, 800);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [programStartDate]);

  useEffect(() => {
    if (programDateStatus !== 'saved') return;
    const t = setTimeout(() => setProgramDateStatus('idle'), 2500);
    return () => clearTimeout(t);
  }, [programDateStatus]);

  const [styleProfile, setStyleProfile] = useState(studio.communicationStyleProfile || '');
  const [savingStyle, setSavingStyle] = useState(false);
  const [styleError, setStyleError] = useState<string | null>(null);
  const [styleSuccess, setStyleSuccess] = useState<string | null>(null);

  async function handleSaveStyle() {
    setStyleError(null);
    setStyleSuccess(null);
    setSavingStyle(true);
    try {
      const res = await updateCommunicationStyle(studio.id, styleProfile);
      if (!res.ok) {
        throw new Error(res.error || 'Failed to save changes');
      }
      setStyleSuccess('Communication style saved!');
      router.refresh();
    } catch (err: any) {
      setStyleError(err.message || 'An error occurred while saving.');
    } finally {
      setSavingStyle(false);
    }
  }

  // Relearn interval — how often (at minimum) the style worker re-learns
  // this studio's profile. Stored server-side in minutes; shown here in
  // whichever unit divides evenly, defaulting to hours (the server default
  // is 240min = 4h).
  const initialIntervalMinutes = studio.styleRefreshIntervalMinutes ?? 240;
  const [intervalUnit, setIntervalUnit] = useState<'minutes' | 'hours'>(
    initialIntervalMinutes % 60 === 0 ? 'hours' : 'minutes',
  );
  const [intervalValue, setIntervalValue] = useState(
    initialIntervalMinutes % 60 === 0 ? initialIntervalMinutes / 60 : initialIntervalMinutes,
  );
  const [intervalStatus, setIntervalStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const [intervalError, setIntervalError] = useState<string | null>(null);
  const intervalFirstRun = useRef(true);

  // Auto-save, same debounced pattern as the Knowledge Base fields above —
  // no separate Save button for this control either.
  useEffect(() => {
    if (intervalFirstRun.current) {
      intervalFirstRun.current = false;
      return;
    }
    setIntervalStatus('saving');
    const timer = setTimeout(async () => {
      try {
        const minutes = intervalUnit === 'hours' ? intervalValue * 60 : intervalValue;
        const res = await updateStyleRefreshInterval(studio.id, minutes);
        if (!res.ok) throw new Error(res.error || 'Failed to save changes');
        setIntervalStatus('saved');
        setIntervalError(null);
        router.refresh();
      } catch (err: any) {
        setIntervalStatus('error');
        setIntervalError(err.message || 'An error occurred while saving.');
      }
    }, 800);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [intervalValue, intervalUnit]);

  useEffect(() => {
    if (intervalStatus !== 'saved') return;
    const t = setTimeout(() => setIntervalStatus('idle'), 2500);
    return () => clearTimeout(t);
  }, [intervalStatus]);

  async function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const filesList = e.target.files;
    if (!filesList || filesList.length === 0) return;

    setError(null);
    setSuccess(null);
    setUploading(true);

    try {
      const uploadPromises = Array.from(filesList).map(async (file) => {
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
        const parseResult = await parseDocument(formData);
        const data = parseResult.data;
        if (!parseResult.ok || !data) {
          throw new Error(parseResult.error || `Failed to extract text from "${file.name}"`);
        }

        return { name: file.name, url: uploadResult.url, text: data.text, platform: 'all' };
      });

      const parsedFiles = await Promise.all(uploadPromises);
      setFiles((prev) => [...prev, ...parsedFiles]);
      setSuccess(`Successfully uploaded and processed ${parsedFiles.length} file(s)!`);
    } catch (err: any) {
      console.error(err);
      setError(err.message || 'Failed to upload and parse files.');
    } finally {
      setUploading(false);
      e.target.value = '';
    }
  }

  function handleRemoveFile(index: number) {
    setError(null);
    setSuccess(null);
    setFiles((prev) => prev.filter((_, i) => i !== index));
  }

  function handleFilePlatform(index: number, platform: string) {
    setFiles((prev) => prev.map((f, i) => i === index ? { ...f, platform } : f));
  }

  return (
    <div className="space-y-6">
      {/* Tab nav */}
      <div className="flex gap-2 border-b border-zinc-200 pb-px dark:border-zinc-800">
        {(
          [
            { id: 'instructions', label: 'Instructions & Documents', icon: Database },
            { id: 'style', label: 'Communication Style', icon: Sparkles },
          ] as const
        ).map((item) => {
          const Icon = item.icon;
          const isActive = activeSection === item.id;
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => setActiveSection(item.id)}
              className={`flex items-center gap-2 rounded-t-lg border-b-2 px-4 py-2.5 text-xs font-bold uppercase tracking-wider transition-colors ${
                isActive
                  ? 'border-[var(--brand,#7c3aed)] text-[color:var(--brand,#7c3aed)]'
                  : 'border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300'
              }`}
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </button>
          );
        })}
      </div>

      {activeSection === 'instructions' && (success || error) && (
        <>
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
        </>
      )}

      {activeSection === 'instructions' && (
      <div className="grid gap-6 md:grid-cols-3">
        {/* Left column */}
        <div className="md:col-span-2 space-y-6">
          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800 flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Database className="h-4 w-4 text-zinc-400" />
                <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Text Instructions</h3>
              </div>
              <AutosaveIndicator status={kbStatus} />
            </div>
            <div className="p-6 space-y-4">
              <div>
                <Label htmlFor="greetingMessage">Greeting Message</Label>
                <textarea
                  id="greetingMessage"
                  className="flex w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm placeholder:text-slate-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:placeholder:text-slate-500 min-h-[80px]"
                  placeholder="Hi! Thanks for reaching out to us. How can we help you today?"
                  value={greeting}
                  onChange={(e) => setGreeting(e.target.value)}
                />
                <FieldHint>Sent automatically on the first message of a new conversation. Use <code>{'{{lead_first_name}}'}</code>, <code>{'{{lead_name}}'}</code>, <code>{'{{studio_name}}'}</code> as placeholders.</FieldHint>
              </div>
              <div>
                <Label htmlFor="knowledgeBase">Instructions / General Info</Label>
                <textarea
                  id="knowledgeBase"
                  className="flex w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm placeholder:text-slate-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:placeholder:text-slate-500 min-h-[250px]"
                  placeholder="Paste FAQs, price details, general studio guidelines here. The AI will read this to answer customer questions..."
                  value={text}
                  onChange={(e) => setText(e.target.value)}
                />
                <FieldHint>Direct textual instructions shown on all channels. Use the document list below to restrict content to specific platforms.</FieldHint>
                {kbStatus === 'error' && kbStatusError && (
                  <p className="mt-1.5 text-xs font-semibold text-rose-600 dark:text-rose-400">{kbStatusError}</p>
                )}
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Uploaded Documents</h3>
            </div>
            <div className="p-6">
              {files.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-10 text-center">
                  <FileText className="h-10 w-10 text-zinc-400 dark:text-zinc-600 mb-2" />
                  <p className="text-sm font-semibold text-zinc-500 dark:text-zinc-400">No documents uploaded yet</p>
                  <p className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">Upload files to feed details directly into the AI.</p>
                </div>
              ) : (
                <div className="divide-y divide-zinc-100 dark:divide-zinc-800">
                  {files.map((file, i) => (
                    <div key={i} className="flex items-center justify-between py-3 gap-3">
                      <div className="flex items-center gap-3 min-w-0 flex-1">
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
                            {file.text ? `${file.text.substring(0, 80)}…` : 'Processing…'}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <PlatformSelect
                          value={file.platform || 'all'}
                          onChange={(v) => handleFilePlatform(i, v)}
                        />
                        <button
                          type="button"
                          onClick={() => handleRemoveFile(i)}
                          className="rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 hover:text-rose-500 dark:hover:bg-zinc-800/50"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div className="flex items-center justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Program Schedule Start Date</h3>
              <AutosaveIndicator status={programDateStatus} />
            </div>
            <div className="p-6">
              <Label htmlFor="programStartDate">Week 1, Day 1 date</Label>
              <input
                id="programStartDate"
                type="date"
                value={programStartDate}
                onChange={(e) => setProgramStartDate(e.target.value)}
                className="mt-1.5 block w-full max-w-xs rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-slate-700 dark:bg-slate-900"
              />
              <FieldHint>
                Only needed if you've uploaded a week-by-week program document (e.g. an "8 Week Challenge" plan).
                Set this to the exact calendar date Week 1's first listed day starts — the AI uses it to work out
                exactly which week/session is "today" instead of guessing. Leave blank if you don't have a
                dated, week-by-week program.
              </FieldHint>
              {programDateStatus === 'error' && programDateError && (
                <p className="mt-1.5 text-xs font-semibold text-rose-600 dark:text-rose-400">{programDateError}</p>
              )}
            </div>
          </div>
        </div>

        {/* Right column */}
        <div className="space-y-6">
          {/* Platform legend */}
          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Domain Guide</h3>
            </div>
            <div className="p-4 space-y-2">
              {DOMAINS.map((p) => (
                <div key={p.value} className="flex items-center gap-2">
                  <span className={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${DOMAIN_COLORS[p.value]}`}>
                    {p.label}
                  </span>
                </div>
              ))}
              <p className="text-xs text-zinc-400 dark:text-zinc-500 pt-2">
                Tag each document with the type of content it covers so the AI can find the most relevant information.
              </p>
            </div>
          </div>

          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Add Documents</h3>
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
                  {uploading ? 'Processing file…' : 'Choose a document'}
                </span>
                <span className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">
                  PDF, Word, PowerPoint, Excel, Text, CSV
                </span>
              </div>
              <FieldHint>
                New files default to All Platforms. Change the platform tag in the document list after uploading.
              </FieldHint>
            </div>
          </div>
        </div>
      </div>
      )}

      {activeSection === 'style' && (
        <div className="max-w-2xl">
          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800 flex items-center gap-2">
              <Sparkles className="h-4 w-4 text-zinc-400" />
              <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Communication Style (learned)</h3>
            </div>
            <div className="p-6 space-y-4">
              {styleSuccess && (
                <div className="flex items-center gap-2 rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-3 text-sm font-semibold text-emerald-600 dark:text-emerald-400">
                  <CheckCircle className="h-4 w-4 shrink-0" />
                  <span>{styleSuccess}</span>
                </div>
              )}
              {styleError && (
                <div className="flex items-center gap-2 rounded-xl border border-rose-500/20 bg-rose-500/5 p-3 text-sm font-semibold text-rose-600 dark:text-rose-400">
                  <AlertCircle className="h-4 w-4 shrink-0" />
                  <span>{styleError}</span>
                </div>
              )}
              <div>
                <Label htmlFor="styleProfile">How your team talks to customers</Label>
                <textarea
                  id="styleProfile"
                  className="flex w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm placeholder:text-slate-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:placeholder:text-slate-500 min-h-[140px]"
                  placeholder="Once your team has sent enough WhatsApp/Instagram replies, this fills in automatically — a short summary of your tone, phrasing, and how you handle pricing questions, learned from your own conversations. Edit it any time; your edit sticks until enough new replies come in to justify a fresh rebuild."
                  value={styleProfile}
                  onChange={(e) => setStyleProfile(e.target.value)}
                />
                <FieldHint>
                  {studio.styleProfileUpdatedAt
                    ? `Last learned ${new Date(studio.styleProfileUpdatedAt).toLocaleDateString()}. Automatically rebuilt as your team sends more replies — the AI also pulls real past replies as examples on top of this summary.`
                    : 'Not learned yet — needs a batch of staff-sent replies first. You can also write this yourself in the meantime.'}
                </FieldHint>
              </div>
              <Button onClick={handleSaveStyle} loading={savingStyle} className="w-full sm:w-auto">
                Save Communication Style
              </Button>

              <div className="border-t border-zinc-200 pt-4 dark:border-zinc-800">
                <div className="flex items-center justify-between gap-2">
                  <Label htmlFor="styleRefreshValue">Relearn at least every</Label>
                  <AutosaveIndicator status={intervalStatus} />
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <input
                    id="styleRefreshValue"
                    type="number"
                    min={intervalUnit === 'hours' ? 1 : 30}
                    max={intervalUnit === 'hours' ? 720 : 43200}
                    value={intervalValue}
                    onChange={(e) => setIntervalValue(Math.max(0, Number(e.target.value) || 0))}
                    className="w-24 rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-slate-700 dark:bg-slate-900"
                  />
                  <select
                    value={intervalUnit}
                    onChange={(e) => setIntervalUnit(e.target.value as 'minutes' | 'hours')}
                    className="rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-slate-700 dark:bg-slate-900"
                  >
                    <option value="minutes">Minutes</option>
                    <option value="hours">Hours</option>
                  </select>
                </div>
                <FieldHint>
                  How often the system checks whether enough new replies have come in to relearn your style —
                  e.g. 4 hours = at most one relearn every 4 hours, only if there&apos;s new material. It never
                  relearns from nothing, no matter how short this is set.
                </FieldHint>
                {intervalStatus === 'error' && intervalError && (
                  <p className="mt-1.5 text-xs font-semibold text-rose-600 dark:text-rose-400">{intervalError}</p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="flex flex-col items-center gap-2">
        <p className="text-xs font-semibold text-zinc-400 dark:text-zinc-500">
          Changes save automatically
        </p>
        <Button
          variant="ghost"
          onClick={() => router.back()}
          className="mx-auto w-full max-w-xs text-center"
        >
          Back
        </Button>
      </div>

      <TestChatDrawer studioId={studio.id} openSignal={testChatSignal} />

      <Dialog open={syncOutcome !== null} onClose={() => setSyncOutcome(null)} widthClassName="max-w-sm">
        <DialogHeader
          icon={
            syncOutcome === 'error' ? (
              <AlertCircle className="h-5 w-5 text-rose-500" />
            ) : (
              <CheckCircle className="h-5 w-5 text-emerald-500" />
            )
          }
          title={syncOutcome === 'error' ? 'Embedding failed' : 'Embedding complete'}
          onClose={() => setSyncOutcome(null)}
        />
        <div className="space-y-4 px-5 py-4">
          <p className="text-sm text-zinc-600 dark:text-zinc-300">
            {syncOutcome === 'error'
              ? "The knowledge base didn't finish embedding — check that the embeddings service is running, then try saving again."
              : 'Your knowledge base has finished processing and is now searchable. You can test how the AI answers using it.'}
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setSyncOutcome(null)}>
              Close
            </Button>
            {syncOutcome === 'complete' && (
              <Button
                leftIcon={<MessageCircle className="h-3.5 w-3.5" />}
                onClick={() => {
                  setSyncOutcome(null);
                  setTestChatSignal((n) => n + 1);
                }}
              >
                Test it now
              </Button>
            )}
          </div>
        </div>
      </Dialog>
    </div>
  );
}
