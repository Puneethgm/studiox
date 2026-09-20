'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { updateStudioSettings } from './actions';
import { SettingsCard, SettingsRow } from './SettingsRows';
import { Plus, Trash2, Clock, X, Calendar, Pencil, Loader2 } from 'lucide-react';

export type AvailabilitySlot = { day: string; times: string[] };

type StudioData = {
  id: string;
  slug: string;
  availabilitySlots?: AvailabilitySlot[];
  availabilityTimezone?: string;
};

const daysOfWeek = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

const commonTimezones = [
  'Asia/Kolkata',
  'Asia/Singapore',
  'Asia/Dubai',
  'Asia/Tokyo',
  'Europe/London',
  'Europe/Paris',
  'America/New_York',
  'America/Chicago',
  'America/Los_Angeles',
  'Australia/Sydney',
  'UTC',
];

function formatTo12Hour(time24: string): string {
  if (!time24) return '';
  const parts = time24.split(':');
  if (parts.length < 2) return time24;
  const hours = parseInt(parts[0] || '0', 10);
  const minutes = parts[1] || '00';
  if (isNaN(hours)) return time24;
  const ampm = hours >= 12 ? 'PM' : 'AM';
  const hours12 = hours % 12 || 12;
  return `${hours12.toString().padStart(2, '0')}:${minutes} ${ampm}`;
}

// Row-based rework of AvailabilitySettings — each weekday is one row (label
// = day, value = "N slots" / "Unavailable" + a pencil that opens the same
// hours-editing popup), instead of a grid of day cards. "Done" in the popup
// now actually persists (the backend takes the whole slots array at once,
// so per-day edits still save all of them together under the hood).
export function AvailabilityRows({ studio, onSaveSuccess }: { studio: StudioData; onSaveSuccess?: (msg: string) => void }) {
  const [slots, setSlots] = useState<AvailabilitySlot[]>(() => {
    const raw = studio.availabilitySlots || [];
    return daysOfWeek.map((day) => {
      const existing = raw.find((s) => (s.day || '').toLowerCase() === day.toLowerCase());
      return { day, times: existing ? existing.times || [] : [] };
    });
  });
  const [timezone, setTimezone] = useState(studio.availabilityTimezone || 'Asia/Kolkata');
  const [editingTimezone, setEditingTimezone] = useState(false);
  const [tzDraft, setTzDraft] = useState(timezone);
  const [tzSaving, setTzSaving] = useState(false);

  const [editingDay, setEditingDay] = useState<string | null>(null);
  const [newTimeInput, setNewTimeInput] = useState('09:00');
  const [modalPage, setModalPage] = useState(1);
  const [modalSaving, setModalSaving] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  async function persist(nextSlots: AvailabilitySlot[], nextTimezone: string) {
    const res = await updateStudioSettings(studio.id, studio.slug, {
      availabilitySlots: nextSlots.map((s) => ({ day: s.day, times: s.times || [] })),
      availabilityTimezone: nextTimezone,
    });
    return res;
  }

  async function saveTimezone() {
    setTzSaving(true);
    const res = await persist(slots, tzDraft);
    setTzSaving(false);
    if (res.ok) {
      setTimezone(tzDraft);
      setEditingTimezone(false);
      onSaveSuccess?.('Timezone updated.');
    }
  }

  const currentEditingSlot = slots.find((s) => s.day === editingDay);
  const itemsPerPage = 5;
  const totalSlotsCount = currentEditingSlot?.times.length || 0;
  const totalPages = Math.ceil(totalSlotsCount / itemsPerPage) || 1;
  const activePage = Math.min(modalPage, totalPages);
  const startIndex = (activePage - 1) * itemsPerPage;
  const paginatedTimes = (currentEditingSlot?.times || []).slice(startIndex, startIndex + itemsPerPage);

  function addTimeToDay(time: string) {
    if (!editingDay || !time) return;
    setSlots((prev) =>
      prev.map((s) => (s.day === editingDay && !s.times.includes(time) ? { ...s, times: [...s.times, time].sort() } : s)),
    );
  }

  function removeTimeFromDay(timeIndex: number) {
    if (!editingDay) return;
    setSlots((prev) => prev.map((s) => (s.day === editingDay ? { ...s, times: s.times.filter((_, i) => i !== timeIndex) } : s)));
  }

  async function onDone() {
    setModalSaving(true);
    setModalError(null);
    const res = await persist(slots, timezone);
    setModalSaving(false);
    if (!res.ok) {
      setModalError(res.error ?? 'Failed to save');
      return;
    }
    onSaveSuccess?.('Availability updated.');
    setEditingDay(null);
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-base font-bold text-zinc-900 dark:text-white">Availability</h3>
        <p className="text-sm text-zinc-400">Weekly booking hours and timezone.</p>
      </div>

      <SettingsCard title="Weekly Schedule">
        {slots.map((slot) => {
          const hasHours = slot.times.length > 0;
          return (
            <SettingsRow key={slot.day} label={slot.day}>
              <div className="flex items-center gap-1.5">
                <span
                  className={`text-xs font-bold ${hasHours ? 'text-zinc-800 dark:text-zinc-100' : 'italic text-zinc-400'}`}
                >
                  {hasHours ? slot.times.map(formatTo12Hour).join(', ') : 'Unavailable'}
                </span>
                <button
                  type="button"
                  onClick={() => {
                    setEditingDay(slot.day);
                    setModalPage(1);
                    setModalError(null);
                  }}
                  className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
                  aria-label={`Edit hours for ${slot.day}`}
                >
                  <Pencil className="h-3.5 w-3.5" />
                </button>
              </div>
            </SettingsRow>
          );
        })}
      </SettingsCard>

      <SettingsCard title="Timezone">
        {editingTimezone ? (
          <SettingsRow label="Operating timezone">
            <div className="flex items-center gap-1.5">
              <select
                autoFocus
                value={tzDraft}
                onChange={(e) => setTzDraft(e.target.value)}
                className="h-8 rounded-lg border border-zinc-200 bg-white px-2 text-xs font-semibold text-zinc-800 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
              >
                {commonTimezones.map((tz) => (
                  <option key={tz} value={tz}>
                    {tz}
                  </option>
                ))}
              </select>
              <button
                type="button"
                onClick={saveTimezone}
                disabled={tzSaving}
                className="shrink-0 rounded-lg p-1.5 text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
              >
                {tzSaving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Clock className="h-3.5 w-3.5" />}
              </button>
              <button
                type="button"
                onClick={() => {
                  setTzDraft(timezone);
                  setEditingTimezone(false);
                }}
                className="shrink-0 rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-900"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          </SettingsRow>
        ) : (
          <SettingsRow label="Operating timezone">
            <div className="flex items-center gap-1.5">
              <span className="text-sm text-zinc-800 dark:text-zinc-100">{timezone}</span>
              <button
                type="button"
                onClick={() => setEditingTimezone(true)}
                className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            </div>
          </SettingsRow>
        )}
        <SettingsRow label="Active workdays">
          <span className="text-sm font-semibold text-zinc-800 dark:text-zinc-100">
            {slots.filter((s) => s.times.length > 0).length} / 7
          </span>
        </SettingsRow>
        <SettingsRow label="Total bookable slots">
          <span className="text-sm font-semibold text-[var(--brand,#7c3aed)]">
            {slots.reduce((acc, s) => acc + s.times.length, 0)}
          </span>
        </SettingsRow>
      </SettingsCard>

      {editingDay && (
        <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="relative w-full max-w-md space-y-6 rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-950">
            <button
              onClick={() => setEditingDay(null)}
              className="absolute right-4 top-4 rounded-lg p-1.5 text-zinc-400 transition-colors hover:text-zinc-700 dark:hover:text-white"
            >
              <X className="h-5 w-5" />
            </button>

            <div className="flex items-center gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-xl bg-[var(--brand,#7c3aed)]/10 text-[var(--brand,#7c3aed)]">
                <Calendar className="h-5 w-5" />
              </div>
              <div>
                <h4 className="text-sm font-black uppercase tracking-wider text-zinc-800 dark:text-white">Hours for {editingDay}</h4>
                <p className="text-[10px] text-zinc-500">Add or remove time slots for this day</p>
              </div>
            </div>

            <div className="space-y-3">
              <label className="block text-sm font-black text-zinc-950 dark:text-white">Current Slots ({totalSlotsCount})</label>
              {paginatedTimes.length > 0 ? (
                <div className="space-y-1.5">
                  <div className="scrollbar-thin max-h-48 space-y-1.5 overflow-y-auto pr-1">
                    {paginatedTimes.map((t, idx) => (
                      <div key={idx} className="flex items-center justify-between rounded-xl border border-zinc-200 bg-zinc-50 p-2.5 dark:border-zinc-800 dark:bg-zinc-900/40">
                        <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200">{formatTo12Hour(t)}</span>
                        <button
                          onClick={() => removeTimeFromDay(startIndex + idx)}
                          className="rounded-lg p-1.5 text-zinc-400 transition-all hover:bg-red-500/10 hover:text-red-500"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                  {totalPages > 1 && (
                    <div className="flex items-center justify-between border-t border-zinc-200 pt-2 dark:border-zinc-800">
                      <button
                        disabled={activePage === 1}
                        onClick={() => setModalPage((p) => Math.max(p - 1, 1))}
                        className="rounded-lg border border-zinc-200 px-2.5 py-1 text-[10px] font-black uppercase text-zinc-500 transition-colors hover:bg-zinc-50 disabled:opacity-40 dark:border-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-900"
                      >
                        Prev
                      </button>
                      <span className="text-[10px] font-black uppercase text-zinc-500 dark:text-zinc-400">
                        Page {activePage} of {totalPages}
                      </span>
                      <button
                        disabled={activePage === totalPages}
                        onClick={() => setModalPage((p) => Math.min(p + 1, totalPages))}
                        className="rounded-lg border border-zinc-200 px-2.5 py-1 text-[10px] font-black uppercase text-zinc-500 transition-colors hover:bg-zinc-50 disabled:opacity-40 dark:border-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-900"
                      >
                        Next
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                <div className="rounded-xl border border-dashed border-zinc-300 bg-zinc-50 py-6 text-center dark:border-zinc-700 dark:bg-zinc-900/40">
                  <span className="text-[10px] font-bold text-zinc-400">No hours configured (Unavailable)</span>
                </div>
              )}
            </div>

            <div className="space-y-2 border-t border-zinc-200 pt-4 dark:border-zinc-800">
              <label className="block text-sm font-black text-zinc-950 dark:text-white">Add Time Slot</label>
              <div className="flex gap-2">
                <input
                  type="time"
                  value={newTimeInput}
                  onChange={(e) => setNewTimeInput(e.target.value)}
                  className="flex-1 rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-xs font-bold text-zinc-800 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200"
                />
                <Button onClick={() => addTimeToDay(newTimeInput)} className="rounded-xl bg-[var(--brand,#7c3aed)] px-4 text-xs font-black uppercase tracking-wider text-white hover:brightness-110">
                  <Plus className="mr-1 h-4 w-4" /> Add Time
                </Button>
              </div>
            </div>

            {modalError && <p className="text-xs font-medium text-red-500">{modalError}</p>}

            <div className="flex justify-end border-t border-zinc-200 pt-4 dark:border-zinc-800">
              <Button
                onClick={onDone}
                loading={modalSaving}
                className="h-10 rounded-xl bg-[var(--brand,#7c3aed)] px-6 text-xs font-black uppercase tracking-widest text-white hover:brightness-110"
              >
                Done
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
