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

  const commonTimezones = [
    "Asia/Kolkata",
    "Asia/Singapore",
    "Asia/Dubai",
    "Asia/Tokyo",
    "Europe/London",
    "Europe/Paris",
    "America/New_York",
    "America/Chicago",
    "America/Los_Angeles",
    "Australia/Sydney",
    "UTC"
  ];

  return (
  return (
    <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
      <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
        <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Availability</h3>
      </div>
      <div className="p-6">
        {/* Timezone selector */}
        <div className="mb-6 max-w-xs">
          <label className="block text-xs font-black uppercase tracking-wider text-zinc-400 mb-2">Timezone</label>
          <select
            className="rounded-xl border border-white/20 px-3 py-1.5 text-xs font-bold bg-white/10 dark:bg-neutral-800/30 dark:border-white/5 focus:outline-none focus:ring-1 focus:ring-brand-500 h-9 w-full text-zinc-800 dark:text-zinc-200"
            value={timezone}
            onChange={(e) => setTimezone(e.target.value)}
          >
            {commonTimezones.map((tz) => (
              <option key={tz} value={tz} className="bg-white dark:bg-neutral-900">{tz}</option>
            ))}
            {timezone && !commonTimezones.includes(timezone) && (
              <option value={timezone} className="bg-white dark:bg-neutral-900">{timezone}</option>
            )}
          </select>
        </div>
        
        {/* Slots list */}
        <div className="space-y-4">
          {slots.map((slot, idx) => (
            <div key={idx} className="p-4 rounded-2xl border border-white/10 bg-white/10 dark:bg-neutral-800/10 flex flex-wrap items-center gap-3">
              <select
                className="rounded-xl border border-white/20 px-3 py-1.5 text-xs font-bold bg-white/10 dark:bg-neutral-800/30 dark:border-white/5 focus:outline-none focus:ring-1 focus:ring-brand-500 h-9"
                value={slot.day}
                onChange={(e) => updateDay(idx, e.target.value)}
              >
                {daysOfWeek.map((d) => (
                  <option key={d} value={d} className="bg-white dark:bg-neutral-900">{d}</option>
                ))}
              </select>

              <div className="flex flex-wrap items-center gap-2">
                {slot.times.map((t, tidx) => (
                  <div key={tidx} className="flex items-center gap-1 bg-white/20 dark:bg-neutral-800/40 border border-white/10 rounded-xl pr-1.5">
                    <Input 
                      type="time" 
                      value={t} 
                      onChange={(e) => updateTime(idx, tidx, e.target.value)} 
                      className="w-[100px] border-none bg-transparent shadow-none h-8 text-xs font-bold focus-visible:ring-0 text-zinc-800 dark:text-zinc-200" 
                    />
                    <Button 
                      variant="ghost" 
                      size="sm" 
                      onClick={() => removeTime(idx, tidx)} 
                      className="h-5 w-5 p-0 text-zinc-400 hover:text-red-500 hover:bg-transparent rounded-md transition-colors"
                    >
                      <XIcon />
                    </Button>
                  </div>
                ))}
              </div>
              
              <Button 
                variant="outline" 
                size="sm" 
                onClick={() => addTime(idx)} 
                className="h-8 text-xs font-black uppercase tracking-wider border-dashed border-white/20 hover:bg-white/10 hover:border-white/30 rounded-xl"
              >
                <Plus className="w-3.5 h-3.5 mr-1 text-brand-500" /> Add Time
              </Button>
              
              <div className="flex-1" />

              <Button 
                variant="ghost" 
                size="sm" 
                onClick={() => removeDaySlot(idx)} 
                className="text-zinc-400 hover:text-red-500 hover:bg-red-500/10 h-8 px-2.5 rounded-xl transition-all"
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          ))}
        </div>

        <div className="mt-4">
          <Button 
            variant="outline" 
            onClick={addDaySlot} 
            className="text-xs font-black uppercase tracking-wider border-white/20 hover:bg-white/10 rounded-xl px-4 py-2"
          >
            <Plus className="w-4 h-4 mr-2 text-brand-500" /> Add Day Slot
          </Button>
        </div>
        
        {error && <p className="text-xs font-black text-red-500 mt-4 uppercase tracking-wider">{error}</p>}
        
        <div className="flex justify-end mt-8 pt-6 border-t border-white/10">
          <Button 
            onClick={onSave} 
            loading={saving} 
            className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
          >
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

