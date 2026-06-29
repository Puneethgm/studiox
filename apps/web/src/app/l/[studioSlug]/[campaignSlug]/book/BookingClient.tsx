'use client';

import { useState, useEffect } from 'react';
import { Calendar, Clock, CheckCircle2, AlertCircle, ArrowRight, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import type { PublicPlan } from '@/lib/public';

interface BookingClientProps {
  leadId: string;
  brandColor: string;
  studioName: string;
  campaignName: string;
  studioSlug: string;
  trialAmountSgd: number;
  availabilitySlots: Record<string, string[]>;
  plans: PublicPlan[];
  paidPlanId?: string;
}

type Step = 'plan' | 'slot' | 'done';

export function BookingClient({
  leadId,
  brandColor,
  studioName,
  campaignName,
  studioSlug,
  availabilitySlots,
  plans,
}: BookingClientProps) {
  const hasPricedPlans = plans.some((p) => p.priceSgd > 0);
  const initialStep: Step = plans.length > 0 ? 'plan' : 'slot';

  const [step, setStep] = useState<Step>(initialStep);
  const [selectedPlanId, setSelectedPlanId] = useState<string>('');
  const [checkoutLoading, setCheckoutLoading] = useState(false);
  const [checkoutError, setCheckoutError] = useState('');

  // Slot state
  const [dates, setDates] = useState<{ dayName: string; dateStr: string; label: string }[]>([]);
  const [selectedDate, setSelectedDate] = useState('');
  const [selectedTime, setSelectedTime] = useState('');
  const [bookLoading, setBookLoading] = useState(false);
  const [bookError, setBookError] = useState('');
  const [submitLocked, setSubmitLocked] = useState(false);
  const [lockSecondsLeft, setLockSecondsLeft] = useState(0);

  // Pre-generate next 7 days (client-only to avoid hydration mismatch)
  useEffect(() => {
    const list: { dayName: string; dateStr: string; label: string }[] = [];
    const weekdays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    for (let i = 0; i < 7; i++) {
      const d = new Date();
      d.setDate(d.getDate() + i);
      if (d.getDay() === 0) continue;
      const dayName = weekdays[d.getDay()] || 'Mon';
      const monthName = months[d.getMonth()] || 'Jan';
      const dateStr = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
      list.push({ dayName, dateStr, label: `${monthName} ${d.getDate()}` });
    }
    setDates(list);
    if (list[0]) setSelectedDate(list[0].dateStr);
  }, []);

  useEffect(() => {
    const last = localStorage.getItem(`last_submit_${leadId}`);
    if (last) {
      const elapsed = Date.now() - parseInt(last, 10);
      if (elapsed < 20000) {
        setSubmitLocked(true);
        setLockSecondsLeft(Math.ceil((20000 - elapsed) / 1000));
      }
    }
  }, [leadId]);

  useEffect(() => {
    if (!submitLocked || lockSecondsLeft <= 0) return;
    const t = setInterval(() => {
      setLockSecondsLeft((prev) => {
        if (prev <= 1) { setSubmitLocked(false); clearInterval(t); return 0; }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(t);
  }, [submitLocked, lockSecondsLeft]);

  const timeSlots = (() => {
    if (!selectedDate) return [];
    const day = new Date(selectedDate).toLocaleDateString('en-US', { weekday: 'long' });
    return availabilitySlots[day] || [];
  })();

  const hasSlots = Object.values(availabilitySlots).some((slots) => slots.length > 0);

  async function handlePlanContinue() {
    if (!selectedPlanId) return;
    const plan = plans.find((p) => p.id === selectedPlanId);
    if (!plan) return;

    if (plan.priceSgd === 0) {
      // Free plan → go straight to slot booking
      setStep('slot');
      return;
    }

    // Paid plan → Stripe checkout
    setCheckoutLoading(true);
    setCheckoutError('');
    try {
      const res = await fetch(`/api/v1/public/studios/${studioSlug}/checkout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ planId: plan.id, leadId }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error || 'Failed to create checkout session');
      }
      const { url } = await res.json();
      window.location.href = url;
    } catch (e: any) {
      setCheckoutError(e.message || 'Something went wrong. Please try again.');
      setCheckoutLoading(false);
    }
  }

  async function handleBook() {
    if (!selectedDate || !selectedTime || bookLoading || submitLocked) return;
    setBookLoading(true);
    setBookError('');
    localStorage.setItem(`last_submit_${leadId}`, Date.now().toString());
    setSubmitLocked(true);
    setLockSecondsLeft(20);
    try {
      const res = await fetch(`/api/v1/public/leads/${encodeURIComponent(leadId)}/trial-slot`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ slot: `${selectedDate} ${selectedTime}` }),
      });
      if (!res.ok) throw new Error('Failed to book slot. Please try again.');
      setStep('done');
    } catch (e: any) {
      setBookError(e.message || 'Something went wrong');
    } finally {
      setBookLoading(false);
    }
  }

  // ── DONE ────────────────────────────────────────────────────────
  if (step === 'done') {
    return (
      <div className="py-12 text-center animate-slide-up">
        <div className="mx-auto mb-6 grid h-16 w-16 place-items-center rounded-full bg-emerald-100 text-emerald-600">
          <CheckCircle2 className="h-10 w-10" />
        </div>
        <h2 className="text-2xl font-black text-slate-900">Booking Confirmed!</h2>
        <p className="mt-3 text-slate-500">Your trial slot is confirmed for:</p>
        <div className="mx-auto mt-5 inline-flex flex-col gap-2 rounded-2xl border border-slate-100 bg-slate-50 px-6 py-4 shadow-sm">
          <div className="flex items-center gap-2 text-sm font-bold text-slate-700">
            <Calendar className="h-4 w-4" style={{ color: brandColor }} />
            {selectedDate}
          </div>
          <div className="flex items-center gap-2 text-sm font-bold text-slate-700">
            <Clock className="h-4 w-4" style={{ color: brandColor }} />
            {selectedTime}
          </div>
        </div>
        <p className="mt-5 text-sm text-slate-400">We&apos;ll send a confirmation to your WhatsApp. See you there!</p>
      </div>
    );
  }

  // ── PLAN SELECTION ────────────────────────────────────────────
  if (step === 'plan') {
    return (
      <div className="space-y-6 animate-slide-up">
        <div>
          <h2 className="text-xl font-black text-slate-900 tracking-tight">Choose a plan</h2>
          <p className="mt-1 text-sm text-slate-500">Select the plan that works best for you.</p>
        </div>

        {checkoutError && (
          <div className="flex items-center gap-2.5 rounded-xl bg-rose-50 p-3 text-sm font-medium text-rose-700">
            <AlertCircle className="h-4 w-4 shrink-0" />
            {checkoutError}
          </div>
        )}

        <div className="space-y-3">
          {plans.map((plan) => {
            const selected = selectedPlanId === plan.id;
            const isFree = plan.priceSgd === 0;
            const priceLabel = isFree
              ? 'Free'
              : `S$${(plan.priceSgd / 100).toFixed(0)}/${plan.billingCycle === 'monthly' ? 'mo' : plan.billingCycle === 'yearly' ? 'yr' : 'time'}`;

            return (
              <button
                key={plan.id}
                type="button"
                onClick={() => setSelectedPlanId(plan.id)}
                className={`w-full text-left rounded-2xl border-2 px-5 py-4 transition-all duration-200 ${
                  selected
                    ? 'shadow-md'
                    : 'border-slate-200 bg-white hover:border-slate-300'
                }`}
                style={selected ? { borderColor: brandColor, background: `${brandColor}08` } : {}}
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-start gap-3 min-w-0">
                    {/* Radio indicator */}
                    <div
                      className="mt-0.5 h-5 w-5 rounded-full border-2 flex-shrink-0 flex items-center justify-center transition-colors"
                      style={selected ? { borderColor: brandColor, background: brandColor } : { borderColor: '#cbd5e1' }}
                    >
                      {selected && <div className="h-2 w-2 rounded-full bg-white" />}
                    </div>
                    <div className="min-w-0">
                      <p className="text-sm font-bold text-slate-800">{plan.planName}</p>
                      <p className="text-xs text-slate-400 capitalize mt-0.5">{plan.billingCycle}</p>
                      {plan.features.length > 0 && (
                        <ul className="mt-2 space-y-1">
                          {plan.features.map((f, i) => (
                            <li key={i} className="flex items-center gap-1.5 text-xs text-slate-500">
                              <CheckCircle2 className="h-3 w-3 text-emerald-500 shrink-0" />
                              {f}
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  </div>
                  <div className="shrink-0 text-right">
                    <span className="text-lg font-black" style={{ color: brandColor }}>{priceLabel}</span>
                  </div>
                </div>
              </button>
            );
          })}
        </div>

        <Button
          type="button"
          disabled={!selectedPlanId || checkoutLoading}
          onClick={handlePlanContinue}
          className="w-full h-12 text-sm font-extrabold flex items-center justify-center gap-2"
          style={{ background: selectedPlanId ? brandColor : undefined }}
        >
          {checkoutLoading ? (
            <><Loader2 className="h-4 w-4 animate-spin" />Redirecting to payment…</>
          ) : (
            <>{plans.find((p) => p.id === selectedPlanId)?.priceSgd === 0 ? 'Continue' : 'Pay & Book'}<ArrowRight className="h-4 w-4" /></>
          )}
        </Button>
      </div>
    );
  }

  // ── SLOT PICKER ───────────────────────────────────────────────
  return (
    <div className="space-y-8 animate-slide-up">
      <div>
        <h2 className="text-xl font-black text-slate-900 tracking-tight">Pick your slot</h2>
        <p className="mt-1 text-sm text-slate-500">Choose a date and time that works for you.</p>
      </div>

      {bookError && (
        <div className="flex items-center gap-2.5 rounded-xl bg-rose-50 p-3 text-sm font-medium text-rose-700">
          <AlertCircle className="h-4 w-4 shrink-0" />
          {bookError}
        </div>
      )}

      {!hasSlots ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 p-6 text-center space-y-2">
          <p className="text-sm font-bold text-amber-800">No availability set up yet</p>
          <p className="text-xs text-amber-600">
            The studio hasn&apos;t added their schedule yet. We&apos;ll reach out to confirm your slot via WhatsApp.
          </p>
          <Button
            type="button"
            onClick={() => setStep('done')}
            className="mt-3 text-sm font-bold"
            style={{ background: brandColor }}
          >
            Confirm anyway
          </Button>
        </div>
      ) : (
        <>
          {/* Date picker */}
          <div>
            <label className="flex items-center gap-2 text-xs font-black uppercase tracking-wider text-slate-400 mb-3">
              <Calendar className="h-3.5 w-3.5" style={{ color: brandColor }} />
              Select date
            </label>
            <div className="grid grid-cols-3 gap-2.5 sm:grid-cols-6">
              {dates.map((d) => {
                const isSelected = selectedDate === d.dateStr;
                return (
                  <button
                    key={d.dateStr}
                    type="button"
                    onClick={() => { setSelectedDate(d.dateStr); setSelectedTime(''); }}
                    className={`flex flex-col items-center rounded-xl border py-3 transition-all ${
                      isSelected ? 'border-transparent shadow-md' : 'border-slate-200 bg-white text-slate-500 hover:border-slate-300'
                    }`}
                    style={isSelected ? { background: brandColor, color: 'white' } : {}}
                  >
                    <span className="text-[10px] font-black uppercase tracking-widest opacity-70">{d.dayName}</span>
                    <span className="mt-0.5 text-base font-black">{d.label.split(' ')[1]}</span>
                    <span className="text-[9px] font-semibold opacity-70">{d.label.split(' ')[0]}</span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Time picker */}
          <div>
            <label className="flex items-center gap-2 text-xs font-black uppercase tracking-wider text-slate-400 mb-3">
              <Clock className="h-3.5 w-3.5" style={{ color: brandColor }} />
              Select time
            </label>
            {timeSlots.length === 0 ? (
              <p className="text-sm text-slate-400 bg-slate-50 rounded-xl p-4 text-center">No classes on this day — try another date.</p>
            ) : (
              <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
                {timeSlots.map((time: string) => {
                  const isSelected = selectedTime === time;
                  return (
                    <button
                      key={time}
                      type="button"
                      onClick={() => setSelectedTime(time)}
                      className={`flex items-center justify-center rounded-xl border py-3 text-xs font-bold transition-all ${
                        isSelected ? 'border-transparent shadow-md' : 'border-slate-200 bg-white text-slate-500 hover:border-slate-300'
                      }`}
                      style={isSelected ? { background: brandColor, color: 'white' } : {}}
                    >
                      {time}
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          <Button
            type="button"
            disabled={!selectedDate || !selectedTime || bookLoading || submitLocked}
            loading={bookLoading}
            onClick={handleBook}
            className="w-full h-12 text-sm font-extrabold"
            style={{ background: brandColor }}
          >
            {submitLocked ? `Locked (${lockSecondsLeft}s)` : 'Confirm Appointment'}
          </Button>
        </>
      )}
    </div>
  );
}
