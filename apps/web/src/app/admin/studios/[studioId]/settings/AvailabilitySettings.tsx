'use client';
import { useState } from "react";
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { updateStudioSettings } from './actions';
import { Plus, Trash2, Clock, X, Calendar } from "lucide-react";

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
    // Ensure all 7 days of the week are always initialized
    return daysOfWeek.map(day => {
      const existing = raw.find((s: any) => (s.day || '').toLowerCase() === day.toLowerCase());
      return {
        day,
        times: existing ? (existing.times || (existing.time ? [existing.time] : [])) : []
      };
    });
  });

  const [timezone, setTimezone] = useState(studio.availabilityTimezone || "Asia/Kolkata");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Modal state
  const [editingDay, setEditingDay] = useState<string | null>(null);
  const [newTimeInput, setNewTimeInput] = useState<string>("09:00");

  const addTimeToDay = (time: string) => {
    if (!editingDay || !time) return;
    setSlots(slots.map(s => {
      if (s.day === editingDay) {
        if (s.times.includes(time)) return s; // Avoid duplicates
        const newTimes = [...s.times, time].sort();
        return { ...s, times: newTimes };
      }
      return s;
    }));
  };

  const removeTimeFromDay = (timeIndex: number) => {
    if (!editingDay) return;
    setSlots(slots.map(s => {
      if (s.day === editingDay) {
        return { ...s, times: s.times.filter((_, i) => i !== timeIndex) };
      }
      return s;
    }));
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
    if (!res.ok) {
      setError(res.error ?? "Failed to save");
    } else {
      alert("Availability settings saved successfully.");
    }
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

  const currentEditingSlot = slots.find(s => s.day === editingDay);

  return (
    <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
      <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
        <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Availability</h3>
      </div>
      
      <div className="p-6 space-y-6">
        {/* Timezone selector */}
        <div className="max-w-xs">
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
        
        {/* 7 Days of the Week Grid */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {slots.map((slot) => {
            const hasHours = slot.times.length > 0;
            return (
              <div 
                key={slot.day} 
                className={`p-4 rounded-2xl border transition-all flex flex-col justify-between h-[160px] ${
                  hasHours 
                    ? 'border-brand-500/30 bg-brand-500/5 dark:border-brand-500/20' 
                    : 'border-white/10 bg-white/5 dark:bg-neutral-800/5'
                }`}
              >
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-xs font-black uppercase tracking-wider text-zinc-800 dark:text-zinc-100">{slot.day}</span>
                    <span className={`text-[10px] font-black uppercase px-2 py-0.5 rounded-full ${
                      hasHours 
                        ? 'bg-brand-500/20 text-brand-600 dark:text-brand-400' 
                        : 'bg-zinc-500/10 text-zinc-500'
                    }`}>
                      {hasHours ? `${slot.times.length} Slots` : 'Unavailable'}
                    </span>
                  </div>
                  
                  {/* Hours preview list */}
                  <div className="flex flex-wrap gap-1 max-h-[60px] overflow-y-auto pr-1 scrollbar-none">
                    {hasHours ? (
                      slot.times.map((t, tidx) => (
                        <span key={tidx} className="text-[10px] font-bold bg-white/30 dark:bg-neutral-800/40 border border-white/10 text-zinc-700 dark:text-zinc-300 px-2 py-0.5 rounded-lg">
                          {t}
                        </span>
                      ))
                    ) : (
                      <span className="text-[10px] font-semibold text-zinc-400 italic">No slots set. Click below to add.</span>
                    )}
                  </div>
                </div>

                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setEditingDay(slot.day)}
                  className="mt-3 w-full h-8 text-[10px] font-black uppercase tracking-wider hover:bg-white/10 dark:hover:bg-neutral-800/40 rounded-xl flex items-center justify-center gap-1 border border-white/5"
                >
                  <Clock className="w-3.5 h-3.5 text-brand-500" /> Manage Hours
                </Button>
              </div>
            );
          })}
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

      {/* Pop-up Modal to Add / Remove hours for a day */}
      {editingDay && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="bg-white dark:bg-neutral-900 border border-white/10 rounded-[28px] max-w-md w-full p-6 space-y-6 shadow-2xl relative animate-in zoom-in-95 duration-200">
            {/* Close Button */}
            <button 
              onClick={() => setEditingDay(null)}
              className="absolute right-4 top-4 p-1.5 text-zinc-400 hover:text-zinc-700 dark:hover:text-white rounded-lg transition-colors"
            >
              <X className="w-5 h-5" />
            </button>

            {/* Header */}
            <div className="flex items-center gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-xl bg-brand-500/10 text-brand-500">
                <Calendar className="w-5 h-5" />
              </div>
              <div>
                <h4 className="text-sm font-black uppercase tracking-wider text-zinc-800 dark:text-white">Hours for {editingDay}</h4>
                <p className="text-[10px] text-zinc-500">Add or remove time slots for this day</p>
              </div>
            </div>

            {/* Current Slots List */}
            <div className="space-y-2">
              <label className="block text-xs font-black uppercase tracking-wider text-zinc-400">Current Slots</label>
              {currentEditingSlot && currentEditingSlot.times.length > 0 ? (
                <div className="max-h-48 overflow-y-auto space-y-1.5 pr-1 scrollbar-thin">
                  {currentEditingSlot.times.map((t, idx) => (
                    <div key={idx} className="flex items-center justify-between p-2.5 rounded-xl border border-white/10 bg-white/10 dark:bg-neutral-800/10">
                      <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200">{t}</span>
                      <button
                        onClick={() => removeTimeFromDay(idx)}
                        className="p-1.5 text-zinc-400 hover:text-red-500 hover:bg-red-500/10 rounded-lg transition-all"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="py-6 text-center border border-dashed border-white/20 rounded-xl bg-white/5">
                  <span className="text-[10px] text-zinc-400 font-bold">No hours configured (Unavailable)</span>
                </div>
              )}
            </div>

            {/* Add New Slot form */}
            <div className="space-y-2 pt-4 border-t border-white/10">
              <label className="block text-xs font-black uppercase tracking-wider text-zinc-400">Add Time Slot</label>
              <div className="flex gap-2">
                <input
                  type="time"
                  value={newTimeInput}
                  onChange={(e) => setNewTimeInput(e.target.value)}
                  className="flex-1 rounded-xl border border-white/20 px-3 py-1.5 text-xs font-bold bg-white/10 dark:bg-neutral-800/30 dark:border-white/5 focus:outline-none focus:ring-1 focus:ring-brand-500 text-zinc-800 dark:text-zinc-200"
                />
                <Button 
                  onClick={() => addTimeToDay(newTimeInput)}
                  className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-wider rounded-xl px-4"
                >
                  <Plus className="w-4 h-4 mr-1" /> Add Time
                </Button>
              </div>
            </div>

            {/* Done Button */}
            <div className="pt-4 border-t border-white/10 flex justify-end">
              <Button 
                onClick={() => setEditingDay(null)}
                className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
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
