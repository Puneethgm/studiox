'use client';
import { useState } from "react";
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { updateStudioSettings } from './actions';
import { Plus, Trash2 } from "lucide-react";

export type AvailabilitySlot = {
  day: string;
  times: string[];
};

const daysOfWeek = [
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
  "Sunday",
];

export function AvailabilitySettings({
  studio,
}: {
  studio: any;
}) {
  const [slots, setSlots] = useState<AvailabilitySlot[]>(() => {
    const raw = studio.availabilitySlots || [];
    return raw.map((s: any) => ({
      day: s.day || "Monday",
      times: s.times || (s.time ? [s.time] : []),
    }));
  });
  const [timezone, setTimezone] = useState(studio.availabilityTimezone || "Asia/Kolkata");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const addDaySlot = () => {
    setSlots([...slots, { day: "Monday", times: ["09:00"] }]);
  };

  const updateDay = (index: number, day: string) => {
    const newSlots = [...slots];
    if (newSlots[index]) {
      newSlots[index].day = day;
      setSlots(newSlots);
    }
  };

  const removeDaySlot = (index: number) => {
    setSlots(slots.filter((_, i) => i !== index));
  };

  const addTime = (index: number) => {
    const newSlots = [...slots];
    if (newSlots[index]) {
      newSlots[index].times = [...newSlots[index].times, "09:00"];
      setSlots(newSlots);
    }
  };

  const updateTime = (dayIndex: number, timeIndex: number, value: string) => {
    const newSlots = [...slots];
    if (newSlots[dayIndex]) {
      const newTimes = [...newSlots[dayIndex].times];
      newTimes[timeIndex] = value;
      newSlots[dayIndex].times = newTimes;
      setSlots(newSlots);
    }
  };

  const removeTime = (dayIndex: number, timeIndex: number) => {
    const newSlots = [...slots];
    if (newSlots[dayIndex]) {
      newSlots[dayIndex].times = newSlots[dayIndex].times.filter((_, i) => i !== timeIndex);
      setSlots(newSlots);
    }
  };

  const onSave = async () => {
    setSaving(true);
    setError(null);
    const sanitizedSlots = slots.map(s => ({
      day: s.day,
      times: s.times || [],
    }));
    const res = await updateStudioSettings(studio.id, studio.slug, {
      name: studio.name || "",
      brandColor: studio.brandColor || "#7c3aed",
      logoUrl: studio.logoUrl || "",
      contactEmail: studio.contactEmail || "",
      active: studio.active ?? true,
      availabilitySlots: sanitizedSlots,
      availabilityTimezone: timezone,
    } as any);
    setSaving(false);
    if (!res.ok) setError(res.error ?? "Failed to save");
  };

  return (
    <div className="mt-8 overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
      <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
        <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Availability</h3>
      </div>
      <div className="p-6">
        {/* Timezone selector */}
        <div className="mb-6">
          <label className="block text-sm font-bold mb-2 text-zinc-700 dark:text-zinc-300">Timezone</label>
          <Input value={timezone} onChange={(e) => setTimezone(e.target.value)} placeholder="Asia/Kolkata" className="max-w-xs" />
        </div>
        
        {/* Slots list */}
        <div className="space-y-4">
          {slots.map((slot, idx) => (
            <div key={idx} className="p-3 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white/50 dark:bg-black/20 flex flex-wrap items-center gap-3">
              <select
                className="rounded-lg border border-zinc-200 px-3 py-1.5 text-sm font-semibold bg-white dark:bg-zinc-900 dark:border-zinc-800 focus:outline-none h-8"
                value={slot.day}
                onChange={(e) => updateDay(idx, e.target.value)}
              >
                {daysOfWeek.map((d) => (
                  <option key={d} value={d}>{d}</option>
                ))}
              </select>

              {slot.times.map((t, tidx) => (
                <div key={tidx} className="flex items-center gap-1 bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-700 rounded-lg pr-1">
                  <Input type="time" value={t} onChange={(e) => updateTime(idx, tidx, e.target.value)} className="w-[110px] border-none shadow-none h-8 text-sm focus-visible:ring-0" />
                  <Button variant="ghost" size="sm" onClick={() => removeTime(idx, tidx)} className="h-6 w-6 p-0 text-zinc-400 hover:text-red-500 rounded-md">
                    <XIcon />
                  </Button>
                </div>
              ))}
              
              <Button variant="outline" size="sm" onClick={() => addTime(idx)} className="h-8 text-xs font-semibold border-dashed border-zinc-300 dark:border-zinc-700">
                <Plus className="w-3 h-3 mr-1" /> Add Time
              </Button>

              <div className="flex-1" />

              <Button variant="ghost" size="sm" onClick={() => removeDaySlot(idx)} className="text-red-500 hover:text-red-600 hover:bg-red-50 h-8 px-2">
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          ))}
        </div>

        <div className="mt-6">
          <Button variant="secondary" onClick={addDaySlot} className="font-semibold shadow-sm">
            <Plus className="w-4 h-4 mr-2" /> Add Day
          </Button>
        </div>

        {error && <p className="text-sm font-semibold text-red-500 mt-4">{error}</p>}
        
        <div className="flex justify-end mt-8 pt-6 border-t border-zinc-200 dark:border-zinc-800">
          <Button onClick={onSave} loading={saving} className="bg-violet-600 hover:bg-violet-700 text-white font-bold shadow-md">
            Save Availability
          </Button>
        </div>
      </div>
    </div>
  );
}

function XIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
  )
}

