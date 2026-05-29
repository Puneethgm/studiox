'use client';
import { useState } from "react";
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { updateStudioSettings } from './actions';
import { Plus, Trash2, Clock, X, Calendar, CheckCircle2 } from "lucide-react";

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

// Helper to format 24-hour HH:MM to 12-hour HH:MM AM/PM
function formatTo12Hour(time24: string): string {
  if (!time24) return "";
  const parts = time24.split(":");
  if (parts.length < 2) return time24;
  const hours = parseInt(parts[0], 10);
  const minutes = parts[1];
  if (isNaN(hours)) return time24;
  const ampm = hours >= 12 ? "PM" : "AM";
  const hours12 = hours % 12 || 12;
  const hoursStr = hours12.toString().padStart(2, "0");
  return `${hoursStr}:${minutes} ${ampm}`;
}

export function AvailabilitySettings({
  studio,
  onSaveSuccess,
}: {
  studio: any;
  onSaveSuccess?: (msg: string) => void;
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
  
  // Modal & Pagination state
  const [editingDay, setEditingDay] = useState<string | null>(null);
  const [newTimeInput, setNewTimeInput] = useState<string>("09:00");
  const [modalPage, setModalPage] = useState<number>(1);

  const openEditModal = (day: string) => {
    setEditingDay(day);
    setModalPage(1); // Reset page to 1
  };

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
      availabilitySlots: sanitizedSlots,
      availabilityTimezone: timezone,
    });
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? "Failed to save");
    } else {
      onSaveSuccess?.("Availability settings saved successfully.");
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

  // Pagination calculations inside modal
  const itemsPerPage = 5;
  const totalSlotsCount = currentEditingSlot?.times.length || 0;
  const totalPages = Math.ceil(totalSlotsCount / itemsPerPage) || 1;
  const activePage = Math.min(modalPage, totalPages);
  const startIndex = (activePage - 1) * itemsPerPage;
  const endIndex = startIndex + itemsPerPage;
  const paginatedTimes = (currentEditingSlot?.times || []).slice(startIndex, endIndex);

  return (
    <>
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Left Column - Weekly Schedule Grid Card */}
        <div className="lg:col-span-2 overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-6 flex flex-col justify-between">
          <div className="space-y-6">
            <div className="border-b border-white/20 pb-4 dark:border-white/5 flex items-center gap-2">
              <Calendar className="h-4 w-4 text-brand-500" />
              <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Weekly Schedule</h3>
            </div>
            
            <div className="grid gap-4 sm:grid-cols-2">
              {slots.map((slot) => {
                const hasHours = slot.times.length > 0;
                return (
                  <div 
                    key={slot.day} 
                    className={`p-4 rounded-2xl border transition-all duration-300 flex flex-col justify-between h-[150px] group hover:-translate-y-1.5 hover:shadow-xl hover:shadow-brand-500/5 ${
                      hasHours 
                        ? 'border-brand-500/30 bg-brand-500/5 dark:border-brand-500/20 hover:border-brand-500 hover:bg-brand-500/10' 
                        : 'border-white/10 bg-white/5 dark:bg-neutral-800/5 hover:border-zinc-400/50 dark:hover:border-zinc-600/50 hover:bg-white/10 dark:hover:bg-neutral-800/10'
                    }`}
                  >
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-black uppercase tracking-wider text-zinc-800 dark:text-zinc-100">{slot.day}</span>
                        <span className={`text-[10px] font-black uppercase px-2 py-0.5 rounded-full transition-colors ${
                          hasHours 
                            ? 'bg-brand-500/20 text-brand-600 dark:text-brand-400' 
                            : 'bg-zinc-500/10 text-zinc-500'
                        }`}>
                          {hasHours ? `${slot.times.length} Slots` : 'Unavailable'}
                        </span>
                      </div>
                      
                      {/* Hours preview list showing 12-hour format */}
                      <div className="flex flex-wrap gap-1 max-h-[52px] overflow-y-auto pr-1 scrollbar-none">
                        {hasHours ? (
                          slot.times.map((t, tidx) => (
                            <span key={tidx} className="text-[10px] font-bold bg-white/30 dark:bg-neutral-800/40 border border-white/10 text-zinc-700 dark:text-zinc-300 px-2 py-0.5 rounded-lg whitespace-nowrap">
                              {formatTo12Hour(t)}
                            </span>
                          ))
                        ) : (
                          <span className="text-[10px] font-semibold text-zinc-400/80 italic">No slots set. Click below to add.</span>
                        )}
                      </div>
                    </div>

                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => openEditModal(slot.day)}
                      className="mt-3 w-full h-8 text-[10px] font-black uppercase tracking-wider hover:bg-white/10 dark:hover:bg-neutral-800/40 rounded-xl flex items-center justify-center gap-1 border border-white/5 transition-all"
                    >
                      <Clock className="w-3.5 h-3.5 text-brand-500 group-hover:scale-110 transition-transform" /> Manage Hours
                    </Button>
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        {/* Right Column - Timezone & Control Card */}
        <div className="lg:col-span-1 overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-6 flex flex-col justify-between h-fit gap-6">
          <div className="space-y-6">
            <div className="border-b border-white/20 pb-4 dark:border-white/5 flex items-center gap-2">
              <Clock className="h-4 w-4 text-brand-500" />
              <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Timezone & Info</h3>
            </div>

            <div>
              <Label className="block text-xs font-black uppercase tracking-wider text-zinc-400 mb-2">Operating Timezone</Label>
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

            <div className="p-4 rounded-2xl border border-white/10 bg-white/5 dark:bg-neutral-800/5 space-y-3">
              <div className="text-xs font-black uppercase tracking-wider text-zinc-400">Schedule Overview</div>
              <div className="flex justify-between text-xs">
                <span className="text-zinc-500">Active Workdays:</span>
                <span className="font-bold text-zinc-800 dark:text-zinc-200">{slots.filter(s => s.times.length > 0).length} / 7</span>
              </div>
              <div className="flex justify-between text-xs">
                <span className="text-zinc-500">Total Bookable Slots:</span>
                <span className="font-bold text-brand-500">{slots.reduce((acc, s) => acc + s.times.length, 0)} slots</span>
              </div>
            </div>
            
            {error && <p className="text-xs font-black text-red-500 uppercase tracking-wider">{error}</p>}
          </div>

          <div className="pt-4 border-t border-white/10 flex justify-end">
            <Button 
              onClick={onSave} 
              loading={saving} 
              className="w-full bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
            >
              Save Schedule
            </Button>
          </div>
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
            <div className="space-y-3">
              <label className="block text-xs font-black uppercase tracking-wider text-zinc-400">Current Slots ({totalSlotsCount})</label>
              
              {paginatedTimes.length > 0 ? (
                <div className="space-y-1.5">
                  <div className="max-h-48 overflow-y-auto space-y-1.5 pr-1 scrollbar-thin">
                    {paginatedTimes.map((t, idx) => (
                      <div key={idx} className="flex items-center justify-between p-2.5 rounded-xl border border-white/10 bg-white/10 dark:bg-neutral-800/10">
                        <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200">
                          {formatTo12Hour(t)}
                        </span>
                        <button
                          onClick={() => {
                            const absoluteIdx = startIndex + idx;
                            removeTimeFromDay(absoluteIdx);
                          }}
                          className="p-1.5 text-zinc-400 hover:text-red-500 hover:bg-red-500/10 rounded-lg transition-all"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    ))}
                  </div>

                  {/* Pagination Controls */}
                  {totalPages > 1 && (
                    <div className="flex items-center justify-between pt-2 border-t border-white/5">
                      <button
                        disabled={activePage === 1}
                        onClick={() => setModalPage(p => Math.max(p - 1, 1))}
                        className="px-2.5 py-1 text-[10px] font-black uppercase border border-white/10 rounded-lg disabled:opacity-40 hover:bg-white/5 transition-colors text-zinc-500 dark:text-zinc-400 disabled:hover:bg-transparent"
                      >
                        Prev
                      </button>
                      <span className="text-[10px] font-black uppercase text-zinc-500 dark:text-zinc-400">
                        Page {activePage} of {totalPages}
                      </span>
                      <button
                        disabled={activePage === totalPages}
                        onClick={() => setModalPage(p => Math.min(p + 1, totalPages))}
                        className="px-2.5 py-1 text-[10px] font-black uppercase border border-white/10 rounded-lg disabled:opacity-40 hover:bg-white/5 transition-colors text-zinc-500 dark:text-zinc-400 disabled:hover:bg-transparent"
                      >
                        Next
                      </button>
                    </div>
                  )}
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
    </>
  );
}
