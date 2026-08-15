'use client';

import { useRef, useState } from 'react';
import { DndContext, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core';
import {
  Plus, Save, CheckCircle2, X, Trash2, Type, Image as ImageIcon, Video, User, Calendar, DollarSign,
  CreditCard, MousePointerClick, Upload, Loader2, AlignLeft, AlignCenter, AlignRight, Eye, Link2,
} from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { api, ApiError } from '@/lib/api';
import type {
  PageBlock, PageBlockType, PageBackground, PageFontFamily, PageTextAlign,
  TextBlockContent, ImageBlockContent, VideoBlockContent, FieldBlockContent, AmountBlockContent, PayButtonBlockContent,
} from '@/lib/types';
import { FONT_FAMILY_STACKS } from '@/lib/types';
import { defaultTrialPageBlocks, defaultTrialPageBackground, computeCanvasHeight, CANVAS_WIDTH } from '@/lib/trialPageDefaults';
import { DraggableBlock } from './DraggableBlock';

interface Toast { id: number; message: string; type: 'success' | 'error'; }

const FONT_OPTIONS: { value: PageFontFamily; label: string }[] = [
  { value: 'system', label: 'System (default)' },
  { value: 'serif', label: 'Serif' },
  { value: 'georgia', label: 'Georgia' },
  { value: 'times', label: 'Times New Roman' },
  { value: 'trebuchet', label: 'Trebuchet MS' },
  { value: 'verdana', label: 'Verdana' },
  { value: 'monospace', label: 'Monospace' },
  { value: 'courier', label: 'Courier New' },
];

// width, height — used by the "Size" quick-preset buttons in the property panel.
const SIZE_PRESETS: Record<'small' | 'medium' | 'large' | 'full', [number, number]> = {
  small: [160, 40],
  medium: [260, 60],
  large: [350, 100],
  full: [350, 200],
};

function newBlockDefaults(type: PageBlockType, zIndex: number): Omit<PageBlock, 'id'> {
  switch (type) {
    case 'text':
      return { type, x: 20, y: 20, width: 300, height: 40, zIndex, content: { text: 'New text', fontSize: 16, color: '#111827', weight: 'normal', fontFamily: 'system', italic: false, underline: false, align: 'left' } satisfies TextBlockContent };
    case 'image':
      return { type, x: 20, y: 20, width: 300, height: 160, zIndex, content: { url: '' } satisfies ImageBlockContent };
    case 'video':
      return { type, x: 20, y: 20, width: 300, height: 180, zIndex, content: { url: '' } satisfies VideoBlockContent };
    case 'name_field':
      return { type, x: 20, y: 20, width: 300, height: 58, zIndex, content: { label: 'Full Name', backgroundColor: '#ffffff', textColor: '#1f2937', labelColor: '#9ca3af' } satisfies FieldBlockContent };
    case 'gender_field':
      return { type, x: 20, y: 20, width: 300, height: 58, zIndex, content: { label: 'Gender', backgroundColor: '#ffffff', textColor: '#1f2937', labelColor: '#9ca3af' } satisfies FieldBlockContent };
    case 'dob_field':
      return { type, x: 20, y: 20, width: 300, height: 58, zIndex, content: { label: 'Date of Birth', backgroundColor: '#ffffff', textColor: '#1f2937', labelColor: '#9ca3af' } satisfies FieldBlockContent };
    case 'amount_display':
      return { type, x: 20, y: 20, width: 300, height: 36, zIndex, content: { label: 'Total due today', backgroundColor: '#f9fafb', textColor: '#111827', labelColor: '#6b7280' } satisfies AmountBlockContent };
    case 'pay_button':
      return { type, x: 20, y: 20, width: 300, height: 50, zIndex, content: { label: 'Pay now', color: '#7c3aed' } satisfies PayButtonBlockContent };
    case 'card_fields':
      return { type, x: 20, y: 20, width: 300, height: 130, zIndex, content: {} };
  }
}

const BLOCK_PALETTE: { type: PageBlockType; label: string; icon: React.ElementType; singleton: boolean }[] = [
  { type: 'text', label: 'Text', icon: Type, singleton: false },
  { type: 'image', label: 'Image', icon: ImageIcon, singleton: false },
  { type: 'video', label: 'Video', icon: Video, singleton: false },
  { type: 'name_field', label: 'Name field', icon: User, singleton: true },
  { type: 'gender_field', label: 'Gender field', icon: User, singleton: true },
  { type: 'dob_field', label: 'Date of birth field', icon: Calendar, singleton: true },
  { type: 'amount_display', label: 'Amount', icon: DollarSign, singleton: true },
  { type: 'card_fields', label: 'Card fields', icon: CreditCard, singleton: true },
  { type: 'pay_button', label: 'Pay button', icon: MousePointerClick, singleton: true },
];

export function TrialPageBuilder({
  studioId,
  studioSlug,
  initialBlocks,
  initialBackground,
}: {
  studioId: string;
  studioSlug: string;
  initialBlocks: PageBlock[] | null;
  initialBackground: PageBackground | null;
}) {
  const [blocks, setBlocks] = useState<PageBlock[]>(
    initialBlocks && initialBlocks.length > 0 ? initialBlocks : defaultTrialPageBlocks(),
  );
  const [background, setBackground] = useState<PageBackground>(initialBackground ?? defaultTrialPageBackground());
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);

  // Require the pointer to move a few pixels before a drag is recognized —
  // otherwise dnd-kit's drag-start machinery can swallow a plain click
  // (0px movement) that was only meant to select the block, making
  // click-to-select unreliable.
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  function showToast(message: string, type: 'success' | 'error' = 'success') {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3000);
  }

  function addBlock(type: PageBlockType) {
    const id = `block-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    const zIndex = blocks.length + 1;
    setBlocks((prev) => [...prev, { id, ...newBlockDefaults(type, zIndex) }]);
    setSelectedId(id);
  }

  function updateBlock(id: string, patch: Partial<PageBlock>) {
    setBlocks((prev) => prev.map((b) => (b.id === id ? { ...b, ...patch } : b)));
  }

  function updateContent(id: string, content: PageBlock['content']) {
    setBlocks((prev) => prev.map((b) => (b.id === id ? { ...b, content } : b)));
  }

  function removeBlock(id: string) {
    setBlocks((prev) => prev.filter((b) => b.id !== id));
    setSelectedId((cur) => (cur === id ? null : cur));
  }

  function handleDragEnd(e: DragEndEvent) {
    const { active, delta } = e;
    if (!delta.x && !delta.y) return;
    setBlocks((prev) =>
      prev.map((b) =>
        b.id === active.id
          ? { ...b, x: Math.max(0, Math.round(b.x + delta.x)), y: Math.max(0, Math.round(b.y + delta.y)) }
          : b,
      ),
    );
  }

  const selected = blocks.find((b) => b.id === selectedId) ?? null;
  const counts = blocks.reduce<Record<string, number>>((acc, b) => {
    acc[b.type] = (acc[b.type] ?? 0) + 1;
    return acc;
  }, {});
  const canvasHeight = computeCanvasHeight(blocks);

  async function save() {
    const cardCount = counts.card_fields ?? 0;
    const payCount = counts.pay_button ?? 0;
    if (cardCount !== 1 || payCount !== 1) {
      showToast('The page needs exactly one Card fields block and one Pay button block.', 'error');
      return;
    }
    setSaving(true);
    try {
      await api(`/api/v1/me/studios/${studioId}/trial-page-layout`, {
        method: 'PUT',
        json: { blocks, background },
      });
      showToast('Trial payment page saved');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Failed to save', 'error');
    } finally {
      setSaving(false);
    }
  }

  const canvasStyle: React.CSSProperties = {
    width: CANVAS_WIDTH,
    height: canvasHeight,
    backgroundColor: background.color,
    backgroundImage: background.imageUrl ? `url(${background.imageUrl})` : undefined,
    backgroundSize: 'cover',
    backgroundPosition: 'center',
  };

  return (
    <div className="flex gap-4 items-start">
      {/* Block palette + page settings */}
      <div className="w-56 shrink-0 space-y-2">
        <Card className="p-3">
          <p className="mb-2 text-xs font-semibold text-gray-500 uppercase tracking-wide">Add a block</p>
          <div className="space-y-1.5">
            {BLOCK_PALETTE.map(({ type, label, icon: Icon, singleton }) => {
              const disabled = singleton && (counts[type] ?? 0) >= 1;
              return (
                <button
                  key={type}
                  disabled={disabled}
                  onClick={() => addBlock(type)}
                  className="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-2.5 py-2 text-xs font-medium text-gray-700 transition-colors hover:border-violet-300 hover:bg-violet-50 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-gray-200 disabled:hover:bg-transparent"
                >
                  <Icon className="h-3.5 w-3.5 text-violet-500" />
                  {label}
                  {disabled && <span className="ml-auto text-[9px] text-gray-400">added</span>}
                </button>
              );
            })}
          </div>
        </Card>

        <Card className="p-3 space-y-2.5">
          <p className="text-xs font-semibold text-gray-500 uppercase tracking-wide">Page background</p>
          <div>
            <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Color</label>
            <input
              type="color"
              value={background.color}
              onChange={(e) => setBackground((b) => ({ ...b, color: e.target.value }))}
              className="mt-1 h-8 w-full rounded border border-gray-200"
            />
          </div>
          <ImageUrlField
            studioId={studioId}
            label="Background image (optional)"
            url={background.imageUrl ?? ''}
            onChange={(url) => setBackground((b) => ({ ...b, imageUrl: url }))}
          />
          {background.imageUrl && (
            <button
              onClick={() => setBackground((b) => ({ ...b, imageUrl: '' }))}
              className="text-[11px] font-medium text-red-500 hover:text-red-600"
            >
              Remove background image
            </button>
          )}
        </Card>

        <p className="px-1 text-[11px] leading-relaxed text-gray-400">
          Exactly one Card fields and one Pay button block are required — everything else is optional and repeatable.
          The page scrolls automatically as you add more blocks below the fold.
        </p>
      </div>

      {/* Canvas */}
      <div className="flex-1 min-w-0">
        <Card className="max-h-[80vh] overflow-auto p-4">
          <div className="mx-auto" style={{ width: CANVAS_WIDTH }}>
            <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
              <div
                onClick={() => setSelectedId(null)}
                className="relative overflow-hidden rounded-lg border border-gray-200"
                style={canvasStyle}
              >
                {blocks.map((block) => (
                  <DraggableBlock
                    key={block.id}
                    block={block}
                    isSelected={block.id === selectedId}
                    onSelect={() => setSelectedId(block.id)}
                    onResize={(width, height) => updateBlock(block.id, { width, height })}
                  />
                ))}
              </div>
            </DndContext>
          </div>
        </Card>
      </div>

      {/* Property panel */}
      {selected && (
        <div className="w-72 shrink-0">
          <Card className="p-4 space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm font-semibold text-slate-800 capitalize">{selected.type.replace('_', ' ')}</p>
              <button onClick={() => setSelectedId(null)} className="text-gray-400 hover:text-gray-600">
                <X className="h-4 w-4" />
              </button>
            </div>

            {selected.type === 'text' && (
              <TextProps content={selected.content as TextBlockContent} onChange={(c) => updateContent(selected.id, c)} />
            )}
            {selected.type === 'image' && (
              <ImageProps studioId={studioId} content={selected.content as ImageBlockContent} onChange={(c) => updateContent(selected.id, c)} />
            )}
            {selected.type === 'video' && (
              <VideoProps studioId={studioId} content={selected.content as VideoBlockContent} onChange={(c) => updateContent(selected.id, c)} />
            )}
            {(selected.type === 'name_field' || selected.type === 'gender_field' || selected.type === 'dob_field') && (
              <FieldProps content={selected.content as FieldBlockContent} onChange={(c) => updateContent(selected.id, c)} />
            )}
            {selected.type === 'amount_display' && (
              <AmountProps content={selected.content as AmountBlockContent} onChange={(c) => updateContent(selected.id, c)} />
            )}
            {selected.type === 'pay_button' && (
              <PayButtonProps content={selected.content as PayButtonBlockContent} onChange={(c) => updateContent(selected.id, c)} />
            )}
            {selected.type === 'card_fields' && (
              <p className="text-xs text-gray-400">The actual card number/expiry/CVC fields render here on the live page — position and size only, no styling to configure.</p>
            )}

            <div>
              <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Size</label>
              <div className="mt-1 flex gap-2">
                <input type="number" value={selected.width} onChange={(e) => updateBlock(selected.id, { width: Number(e.target.value) })} className="w-1/2 rounded border border-gray-200 px-2 py-1.5 text-sm" placeholder="width" />
                <input type="number" value={selected.height} onChange={(e) => updateBlock(selected.id, { height: Number(e.target.value) })} className="w-1/2 rounded border border-gray-200 px-2 py-1.5 text-sm" placeholder="height" />
              </div>
              <div className="mt-2 grid grid-cols-4 gap-1.5">
                {(Object.entries(SIZE_PRESETS) as [keyof typeof SIZE_PRESETS, [number, number]][]).map(([key, [w, h]]) => (
                  <button
                    key={key}
                    onClick={() => updateBlock(selected.id, { width: w, height: h })}
                    className="rounded border border-gray-200 py-1 text-[10px] font-semibold uppercase tracking-wide text-gray-500 hover:border-violet-300 hover:bg-violet-50 hover:text-violet-600"
                  >
                    {key}
                  </button>
                ))}
              </div>
              <p className="mt-1 text-[10px] text-gray-400">Or drag the violet handle on the block&apos;s corner to resize freely.</p>
            </div>

            {(selected.type !== 'card_fields' && selected.type !== 'pay_button') && (
              <button
                onClick={() => removeBlock(selected.id)}
                className="flex items-center gap-1.5 text-xs font-medium text-red-500 hover:text-red-600"
              >
                <Trash2 className="h-3.5 w-3.5" />
                Delete this block
              </button>
            )}
            {(selected.type === 'card_fields' || selected.type === 'pay_button') && (
              <p className="text-[11px] text-gray-400">This block is required and can&apos;t be deleted — add a new one first if you want to replace it.</p>
            )}
          </Card>
        </div>
      )}

      <div className="fixed bottom-5 right-5 z-50 flex flex-col items-end gap-2">
        <div className="flex items-center gap-2">
          {studioSlug && (
            <a
              href={`/trial-details/preview?studio=${encodeURIComponent(studioSlug)}`}
              target="_blank"
              rel="noopener noreferrer"
              title="Opens the last SAVED version in a new tab — save first to preview unsaved changes"
              className="inline-flex h-10 items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-600 shadow-sm hover:border-violet-300 hover:text-violet-600"
            >
              <Eye className="h-4 w-4" />
              Preview
            </a>
          )}
          {studioSlug && (
            <button
              onClick={() => {
                const url = `${window.location.origin}/trial-details/studio/${encodeURIComponent(studioSlug)}`;
                navigator.clipboard.writeText(url);
                showToast('Static signup link copied — safe to send manually, no lead needed');
              }}
              title="A static link (no lead attached) you can send manually — anyone who pays through it becomes a new lead automatically"
              className="inline-flex h-10 items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-600 shadow-sm hover:border-violet-300 hover:text-violet-600"
            >
              <Link2 className="h-4 w-4" />
              Copy static link
            </button>
          )}
          <Button onClick={save} disabled={saving}>
            <Save className="h-4 w-4" />
            {saving ? 'Saving…' : 'Save page'}
          </Button>
        </div>
        <div className="flex flex-col gap-2 pointer-events-none">
          {toasts.map((t) => (
            <div
              key={t.id}
              className={`flex items-center gap-3 rounded-xl px-4 py-3 shadow-lg text-sm font-medium pointer-events-auto transition-all border
                ${t.type === 'success' ? 'bg-white text-slate-800 border-slate-200' : 'bg-white text-red-600 border-red-200'}`}
            >
              {t.type === 'success' ? <CheckCircle2 className="h-4 w-4 shrink-0 text-brand-500" /> : <X className="h-4 w-4 shrink-0 text-red-500" />}
              {t.message}
              <button className="ml-2 opacity-70 hover:opacity-100" onClick={() => setToasts((prev) => prev.filter((x) => x.id !== t.id))}>
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function TextProps({ content, onChange }: { content: TextBlockContent; onChange: (c: TextBlockContent) => void }) {
  return (
    <div className="space-y-3">
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Text</label>
        <textarea value={content.text} onChange={(e) => onChange({ ...content, text: e.target.value })} rows={3} className="mt-1 w-full rounded border border-gray-200 px-2.5 py-1.5 text-sm resize-y" />
      </div>
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Font</label>
        <select
          value={content.fontFamily ?? 'system'}
          onChange={(e) => onChange({ ...content, fontFamily: e.target.value as TextBlockContent['fontFamily'] })}
          className="mt-1 w-full rounded border border-gray-200 px-2.5 py-1.5 text-sm"
        >
          {FONT_OPTIONS.map((f) => (
            <option key={f.value} value={f.value} style={{ fontFamily: FONT_FAMILY_STACKS[f.value] }}>{f.label}</option>
          ))}
        </select>
      </div>
      <div className="flex gap-2">
        <div className="flex-1">
          <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Size</label>
          <input type="number" value={content.fontSize} onChange={(e) => onChange({ ...content, fontSize: Number(e.target.value) })} className="mt-1 w-full rounded border border-gray-200 px-2.5 py-1.5 text-sm" />
        </div>
        <div className="flex-1">
          <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Color</label>
          <input type="color" value={content.color} onChange={(e) => onChange({ ...content, color: e.target.value })} className="mt-1 h-9 w-full rounded border border-gray-200" />
        </div>
      </div>
      <div className="flex items-center gap-4">
        <label className="flex items-center gap-1.5 text-xs text-gray-600">
          <input type="checkbox" checked={content.weight === 'bold'} onChange={(e) => onChange({ ...content, weight: e.target.checked ? 'bold' : 'normal' })} />
          Bold
        </label>
        <label className="flex items-center gap-1.5 text-xs text-gray-600">
          <input type="checkbox" checked={!!content.italic} onChange={(e) => onChange({ ...content, italic: e.target.checked })} />
          Italic
        </label>
        <label className="flex items-center gap-1.5 text-xs text-gray-600">
          <input type="checkbox" checked={!!content.underline} onChange={(e) => onChange({ ...content, underline: e.target.checked })} />
          Underline
        </label>
      </div>
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Alignment</label>
        <div className="mt-1 grid grid-cols-3 gap-1.5">
          {([['left', AlignLeft], ['center', AlignCenter], ['right', AlignRight]] as [PageTextAlign, React.ElementType][]).map(([align, Icon]) => (
            <button
              key={align}
              onClick={() => onChange({ ...content, align })}
              className={`flex items-center justify-center rounded border py-1.5 ${(content.align ?? 'left') === align ? 'border-violet-300 bg-violet-50 text-violet-600' : 'border-gray-200 text-gray-400 hover:border-violet-200'}`}
            >
              <Icon className="h-3.5 w-3.5" />
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

/** Shared "paste a URL or upload a file" control — used by the image and
 * video blocks' property panels and the page-background setting. */
function ImageUrlField({ studioId, label, url, onChange, kind = 'image' }: {
  studioId: string; label: string; url: string; onChange: (url: string) => void; kind?: 'image' | 'video';
}) {
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function handleFile(file: File) {
    setUploading(true);
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(`/api/v1/studios/${studioId}/messaging/upload`, { method: 'POST', body: form });
      if (!res.ok) throw new Error('Upload failed');
      const data = await res.json();
      onChange(data.url);
    } catch {
      // Silently leave the URL field as-is — the admin can still paste a URL by hand.
    } finally {
      setUploading(false);
    }
  }

  return (
    <div>
      <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">{label}</label>
      <div className="mt-1 flex gap-1.5">
        <input value={url} onChange={(e) => onChange(e.target.value)} placeholder="https://…" className="min-w-0 flex-1 rounded border border-gray-200 px-2.5 py-1.5 text-sm" />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading}
          title="Upload a file"
          className="flex shrink-0 items-center justify-center rounded border border-gray-200 px-2.5 text-gray-500 hover:border-violet-300 hover:bg-violet-50 hover:text-violet-600 disabled:opacity-50"
        >
          {uploading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Upload className="h-3.5 w-3.5" />}
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept={kind === 'video' ? 'video/*' : 'image/*'}
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) handleFile(file);
            e.target.value = '';
          }}
        />
      </div>
      {url && kind === 'image' && (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={url} alt="" className="mt-2 h-16 w-full rounded border border-gray-100 object-cover" />
      )}
      {url && kind === 'video' && (
        <video src={url} muted className="mt-2 h-16 w-full rounded border border-gray-100 object-cover" />
      )}
    </div>
  );
}

function ImageProps({ studioId, content, onChange }: { studioId: string; content: ImageBlockContent; onChange: (c: ImageBlockContent) => void }) {
  return <ImageUrlField studioId={studioId} label="Image" url={content.url} onChange={(url) => onChange({ url })} />;
}

function VideoProps({ studioId, content, onChange }: { studioId: string; content: VideoBlockContent; onChange: (c: VideoBlockContent) => void }) {
  return (
    <div>
      <ImageUrlField studioId={studioId} label="Video" url={content.url} onChange={(url) => onChange({ url })} kind="video" />
      <p className="mt-1.5 text-[11px] text-gray-400">Plays muted and looped automatically, like a silent background clip.</p>
    </div>
  );
}

function FieldProps({ content, onChange }: { content: FieldBlockContent; onChange: (c: FieldBlockContent) => void }) {
  return (
    <div className="space-y-3">
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Label</label>
        <input value={content.label} onChange={(e) => onChange({ ...content, label: e.target.value })} className="mt-1 w-full rounded border border-gray-200 px-2.5 py-1.5 text-sm" />
      </div>
      <div className="grid grid-cols-3 gap-2">
        <div>
          <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Box</label>
          <input type="color" value={content.backgroundColor ?? '#ffffff'} onChange={(e) => onChange({ ...content, backgroundColor: e.target.value })} className="mt-1 h-8 w-full rounded border border-gray-200" />
        </div>
        <div>
          <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Text</label>
          <input type="color" value={content.textColor ?? '#1f2937'} onChange={(e) => onChange({ ...content, textColor: e.target.value })} className="mt-1 h-8 w-full rounded border border-gray-200" />
        </div>
        <div>
          <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Label</label>
          <input type="color" value={content.labelColor ?? '#9ca3af'} onChange={(e) => onChange({ ...content, labelColor: e.target.value })} className="mt-1 h-8 w-full rounded border border-gray-200" />
        </div>
      </div>
    </div>
  );
}

function AmountProps({ content, onChange }: { content: AmountBlockContent; onChange: (c: AmountBlockContent) => void }) {
  return (
    <div className="space-y-3">
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Label</label>
        <input value={content.label} onChange={(e) => onChange({ ...content, label: e.target.value })} className="mt-1 w-full rounded border border-gray-200 px-2.5 py-1.5 text-sm" />
        <p className="mt-1 text-[11px] text-gray-400">The actual amount is always shown live — this just labels it.</p>
      </div>
      <div className="grid grid-cols-3 gap-2">
        <div>
          <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Box</label>
          <input type="color" value={content.backgroundColor ?? '#f9fafb'} onChange={(e) => onChange({ ...content, backgroundColor: e.target.value })} className="mt-1 h-8 w-full rounded border border-gray-200" />
        </div>
        <div>
          <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Amount</label>
          <input type="color" value={content.textColor ?? '#111827'} onChange={(e) => onChange({ ...content, textColor: e.target.value })} className="mt-1 h-8 w-full rounded border border-gray-200" />
        </div>
        <div>
          <label className="text-[10px] font-medium text-gray-400 uppercase tracking-wide">Label</label>
          <input type="color" value={content.labelColor ?? '#6b7280'} onChange={(e) => onChange({ ...content, labelColor: e.target.value })} className="mt-1 h-8 w-full rounded border border-gray-200" />
        </div>
      </div>
    </div>
  );
}

function PayButtonProps({ content, onChange }: { content: PayButtonBlockContent; onChange: (c: PayButtonBlockContent) => void }) {
  return (
    <div className="space-y-3">
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Label</label>
        <input value={content.label} onChange={(e) => onChange({ ...content, label: e.target.value })} className="mt-1 w-full rounded border border-gray-200 px-2.5 py-1.5 text-sm" />
      </div>
      <div>
        <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Color</label>
        <input type="color" value={content.color} onChange={(e) => onChange({ ...content, color: e.target.value })} className="mt-1 h-9 w-full rounded border border-gray-200" />
      </div>
    </div>
  );
}
