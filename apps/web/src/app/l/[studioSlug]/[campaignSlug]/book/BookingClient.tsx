'use client';

import { useState, useEffect } from 'react';
import { Calendar, Clock, CheckCircle2, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';

interface BookingClientProps {
  leadId: string;
  brandColor: string;
  studioName: string;
  campaignName: string;
}

export function BookingClient({ leadId, brandColor, studioName, campaignName }: BookingClientProps) {
  const [dates, setDates] = useState<{ dayName: string; dateStr: string; label: string }[]>([]);
  const [selectedDate, setSelectedDate] = useState<string>('');
  const [selectedTime, setSelectedTime] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState('');
  const [submitLocked, setSubmitLocked] = useState(false);
  const [lockSecondsLeft, setLockSecondsLeft] = useState(0);

  // Pre-generate next 7 days on mount to avoid server-client hydration mismatches
  useEffect(() => {
    const list: { dayName: string; dateStr: string; label: string }[] = [];
    const weekdays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

    for (let i = 0; i < 7; i++) {
      const d = new Date();
      d.setDate(d.getDate() + i);
      
      // Skip Sundays for bookings
      if (d.getDay() === 0) continue;

      const dayName = weekdays[d.getDay()] || 'Mon';
      const monthName = months[d.getMonth()] || 'Jan';
      const dayNum = d.getDate();
      
      const dateStr = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
      const label = `${monthName} ${dayNum}`;
      
      list.push({ dayName, dateStr, label });
    }
    setDates(list);
    if (list.length > 0 && list[0]) {
      setSelectedDate(list[0].dateStr);
    }
  }, []);

  // Cooldown check on mount
  useEffect(() => {
    const lastSubmit = localStorage.getItem(`last_submit_${leadId}`);
    if (lastSubmit) {
      const elapsed = Date.now() - parseInt(lastSubmit, 10);
      if (elapsed < 20000) {
        setSubmitLocked(true);
        setLockSecondsLeft(Math.ceil((20000 - elapsed) / 1000));
      }
    }
  }, [leadId]);

  // Cooldown countdown timer
  useEffect(() => {
    if (!submitLocked || lockSecondsLeft <= 0) return;
    const timer = setInterval(() => {
      setLockSecondsLeft((prev) => {
        if (prev <= 1) {
          setSubmitLocked(false);
          clearInterval(timer);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, [submitLocked, lockSecondsLeft]);

  const timeSlots = [
    '09:00 AM',
    '10:30 AM',
    '12:00 PM',
    '02:30 PM',
    '04:00 PM',
    '05:30 PM',
    '07:00 PM'
  ];

  async function handleBook() {
    if (!selectedDate || !selectedTime || loading || submitLocked) return;
    setLoading(true);
    setError('');

    // Lock immediately to prevent multiple rapid submissions
    localStorage.setItem(`last_submit_${leadId}`, Date.now().toString());
    setSubmitLocked(true);
    setLockSecondsLeft(20);

    try {
      const formattedSlot = `${selectedDate} ${selectedTime}`;
      const res = await fetch(`/api/v1/public/leads/${encodeURIComponent(leadId)}/trial-slot`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ slot: formattedSlot }),
      });

      if (!res.ok) {
        throw new Error('Failed to book slot. Please try again.');
      }

      setSuccess(true);
    } catch (err: any) {
      setError(err.message || 'Something went wrong');
    } finally {
      setLoading(false);
    }
  }

  if (success) {
    return (
      <div className="py-12 text-center animate-slide-up">
        <div className="mx-auto mb-6 grid h-16 w-16 place-items-center rounded-full bg-emerald-100 text-emerald-600 dark:bg-emerald-950/30 dark:text-emerald-400">
          <CheckCircle2 className="h-10 w-10 animate-bounce" />
        </div>
        <h2 className="text-3xl font-extrabold text-slate-900 dark:text-white">Booking Confirmed!</h2>
        <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
          Thank you! Your trial slot is confirmed for:
        </p>
        <div 
          className="mx-auto mt-6 inline-flex flex-col gap-2 rounded-2xl border border-white/20 bg-white/50 px-6 py-4 shadow-lg backdrop-blur-md dark:border-white/5 dark:bg-white/5"
        >
          <div className="flex items-center justify-center gap-2 text-sm font-bold text-slate-700 dark:text-slate-300">
            <Calendar className="h-4 w-4" style={{ color: brandColor }} />
            <span>{selectedDate}</span>
          </div>
          <div className="flex items-center justify-center gap-2 text-sm font-bold text-slate-700 dark:text-slate-300">
            <Clock className="h-4 w-4" style={{ color: brandColor }} />
            <span>{selectedTime}</span>
          </div>
        </div>
        <p className="mt-6 text-sm font-medium text-slate-400">
          We have sent a confirmation message to your WhatsApp. See you at the center!
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-8 animate-slide-up">
      {error && (
        <div className="flex items-center gap-2.5 rounded-xl bg-rose-50 p-4 text-sm font-medium text-rose-800 dark:bg-rose-950/20 dark:text-rose-400">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Date Picker */}
      <div>
        <label className="flex items-center gap-2 text-sm font-black uppercase tracking-wider text-slate-500 dark:text-slate-400">
          <Calendar className="h-4 w-4" style={{ color: brandColor }} />
          <span>1. Select Date</span>
        </label>
        
        <div className="mt-4 grid grid-cols-3 gap-3 sm:grid-cols-6">
          {dates.map((d) => {
            const isSelected = selectedDate === d.dateStr;
            return (
              <button
                key={d.dateStr}
                type="button"
                onClick={() => setSelectedDate(d.dateStr)}
                className={`relative flex flex-col items-center justify-center rounded-2xl border p-3.5 transition-all duration-300 ${
                  isSelected
                    ? 'border-transparent bg-white text-slate-900 shadow-xl'
                    : 'border-slate-200 bg-white/40 text-slate-500 hover:border-slate-300 dark:border-white/5 dark:bg-white/5 dark:text-slate-400'
                }`}
                style={isSelected ? { outline: `2px solid ${brandColor}` } : {}}
              >
                <span className="text-[10px] font-black uppercase tracking-widest opacity-60">
                  {d.dayName}
                </span>
                <span className="mt-1 text-sm font-extrabold">
                  {d.label.split(' ')[1]}
                </span>
                <span className="text-[9px] font-bold opacity-75">
                  {d.label.split(' ')[0]}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Time Picker */}
      <div>
        <label className="flex items-center gap-2 text-sm font-black uppercase tracking-wider text-slate-500 dark:text-slate-400">
          <Clock className="h-4 w-4" style={{ color: brandColor }} />
          <span>2. Select Time Slot</span>
        </label>
        
        <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {timeSlots.map((time) => {
            const isSelected = selectedTime === time;
            return (
              <button
                key={time}
                type="button"
                onClick={() => setSelectedTime(time)}
                className={`flex items-center justify-center rounded-xl border py-3 text-xs font-bold transition-all duration-300 ${
                  isSelected
                    ? 'border-transparent bg-white text-slate-900 shadow-lg'
                    : 'border-slate-200 bg-white/40 text-slate-500 hover:border-slate-300 dark:border-white/5 dark:bg-white/5 dark:text-slate-400'
                }`}
                style={isSelected ? { outline: `2px solid ${brandColor}` } : {}}
              >
                {time}
              </button>
            );
          })}
        </div>
      </div>

      {/* Submit */}
      <div className="pt-4 border-t border-slate-200/50 dark:border-white/5">
        <Button
          type="button"
          disabled={!selectedDate || !selectedTime || loading || submitLocked}
          loading={loading}
          onClick={handleBook}
          className="w-full h-12 text-sm font-extrabold shadow-xl hover:scale-[1.01] active:scale-[0.99] transition-all"
          style={{ background: brandColor }}
        >
          {submitLocked ? `Locked (${lockSecondsLeft}s)` : 'Confirm Appointment'}
        </Button>
      </div>
    </div>
  );
}
