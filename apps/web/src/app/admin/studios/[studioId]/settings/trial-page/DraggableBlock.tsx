'use client';

import { useRef } from 'react';
import { useDraggable } from '@dnd-kit/core';
import { CreditCard, User, Calendar, DollarSign, Image as ImageIcon, Video as VideoIcon, GripHorizontal } from 'lucide-react';
import { cn } from '@/lib/cn';
import type {
  PageBlock, TextBlockContent, ImageBlockContent, VideoBlockContent, FieldBlockContent,
  AmountBlockContent, PayButtonBlockContent,
} from '@/lib/types';
import { FONT_FAMILY_STACKS } from '@/lib/types';

function BlockPreview({ block }: { block: PageBlock }) {
  switch (block.type) {
    case 'text': {
      const c = block.content as TextBlockContent;
      return (
        <div
          className="h-full w-full truncate px-1"
          style={{
            fontSize: c.fontSize,
            color: c.color,
            fontWeight: c.weight === 'bold' ? 700 : 400,
            fontStyle: c.italic ? 'italic' : 'normal',
            textDecoration: c.underline ? 'underline' : 'none',
            textAlign: c.align ?? 'left',
            fontFamily: FONT_FAMILY_STACKS[c.fontFamily] ?? FONT_FAMILY_STACKS.system,
          }}
        >
          {c.text || 'Text block'}
        </div>
      );
    }
    case 'image': {
      const c = block.content as ImageBlockContent;
      return c.url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={c.url} alt="" className="h-full w-full object-cover" draggable={false} />
      ) : (
        <div className="flex h-full w-full items-center justify-center bg-gray-100 text-gray-400">
          <ImageIcon className="h-6 w-6" />
        </div>
      );
    }
    case 'video': {
      const c = block.content as VideoBlockContent;
      return c.url ? (
        <video src={c.url} muted className="h-full w-full object-cover" />
      ) : (
        <div className="flex h-full w-full items-center justify-center bg-gray-100 text-gray-400">
          <VideoIcon className="h-6 w-6" />
        </div>
      );
    }
    case 'name_field':
    case 'gender_field':
    case 'dob_field': {
      const c = block.content as FieldBlockContent;
      const Icon = block.type === 'dob_field' ? Calendar : User;
      return (
        <div className="flex h-full w-full flex-col justify-center gap-1 px-2">
          <span className="text-[9px] font-bold uppercase tracking-wide" style={{ color: c.labelColor ?? '#9ca3af' }}>{c.label}</span>
          <div
            className="flex items-center gap-1.5 rounded border border-gray-300 px-2 py-1.5"
            style={{ backgroundColor: c.backgroundColor ?? '#ffffff' }}
          >
            <Icon className="h-3 w-3" style={{ color: c.textColor ?? '#1f2937', opacity: 0.5 }} />
            <span className="text-xs" style={{ color: c.textColor ?? '#1f2937', opacity: 0.5 }}>…</span>
          </div>
        </div>
      );
    }
    case 'amount_display': {
      const c = block.content as AmountBlockContent;
      return (
        <div className="flex h-full w-full items-center justify-between rounded px-2.5" style={{ backgroundColor: c.backgroundColor ?? '#f9fafb' }}>
          <span className="text-xs font-semibold" style={{ color: c.labelColor ?? '#6b7280' }}>{c.label}</span>
          <span className="flex items-center gap-1 text-xs font-black" style={{ color: c.textColor ?? '#9ca3af' }}>
            <DollarSign className="h-3 w-3" />dynamic
          </span>
        </div>
      );
    }
    case 'pay_button': {
      const c = block.content as PayButtonBlockContent;
      return (
        <button
          type="button"
          className="h-full w-full rounded text-xs font-bold text-white"
          style={{ background: c.color }}
        >
          {c.label || 'Pay'}
        </button>
      );
    }
    case 'card_fields':
      return (
        <div className="flex h-full w-full flex-col items-center justify-center gap-1 rounded border-2 border-dashed border-violet-200 bg-violet-50/50 text-violet-400">
          <CreditCard className="h-5 w-5" />
          <span className="text-[10px] font-bold uppercase tracking-wide">Secure card fields</span>
        </div>
      );
    default:
      return null;
  }
}

export function DraggableBlock({
  block,
  isSelected,
  onSelect,
  onResize,
}: {
  block: PageBlock;
  isSelected: boolean;
  onSelect: () => void;
  onResize: (width: number, height: number) => void;
}) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: block.id });
  const resizing = useRef<{ startX: number; startY: number; startW: number; startH: number } | null>(null);

  const style: React.CSSProperties = {
    position: 'absolute',
    left: block.x,
    top: block.y,
    width: block.width,
    height: block.height,
    zIndex: block.zIndex,
    transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
  };

  function onResizeStart(e: React.PointerEvent) {
    e.stopPropagation();
    e.preventDefault();
    resizing.current = { startX: e.clientX, startY: e.clientY, startW: block.width, startH: block.height };
    function onMove(ev: PointerEvent) {
      if (!resizing.current) return;
      const dx = ev.clientX - resizing.current.startX;
      const dy = ev.clientY - resizing.current.startY;
      onResize(Math.max(40, resizing.current.startW + dx), Math.max(24, resizing.current.startH + dy));
    }
    function onUp() {
      resizing.current = null;
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
    }
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      onClick={(e) => {
        e.stopPropagation();
        onSelect();
      }}
      className={cn(
        'cursor-move touch-none overflow-hidden rounded border-2 bg-white',
        isSelected ? 'border-violet-500' : 'border-transparent hover:border-violet-200',
      )}
    >
      <BlockPreview block={block} />
      {isSelected && (
        <div
          onPointerDown={onResizeStart}
          title="Drag to resize"
          className="nodrag absolute -bottom-2 -right-2 z-10 flex h-7 w-7 cursor-se-resize items-center justify-center rounded-full border-2 border-white bg-violet-500 text-white shadow-md hover:bg-violet-600"
        >
          <GripHorizontal className="h-3.5 w-3.5 rotate-45" />
        </div>
      )}
    </div>
  );
}
