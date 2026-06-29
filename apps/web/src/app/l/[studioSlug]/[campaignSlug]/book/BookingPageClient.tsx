'use client';

import { useState, useEffect, useRef } from 'react';
import { loadStripe } from '@stripe/stripe-js';
import { Elements, CardNumberElement, CardExpiryElement, CardCvcElement, useStripe, useElements } from '@stripe/react-stripe-js';
import { CheckCircle2, Calendar, Clock, AlertCircle, Loader2, ShoppingCart, Shield, Lock } from 'lucide-react';
import type { PublicStudio, PublicPlan } from '@/lib/public';

type Step = 'checkout' | 'slot' | 'done';
interface Props { studio: PublicStudio; plans: PublicPlan[]; leadId: string; studioSlug: string; campaignSlug: string; availabilitySlots: Record<string, string[]>; startAtSlot?: boolean; }

const COUNTRY_CODES = [
  { flag: '🇸🇬', code: '+65' },
  { flag: '🇺🇸', code: '+1' },
  { flag: '🇬🇧', code: '+44' },
  { flag: '🇮🇳', code: '+91' },
  { flag: '🇦🇺', code: '+61' },
  { flag: '🇲🇾', code: '+60' },
  { flag: '🇵🇭', code: '+63' },
  { flag: '🇮🇩', code: '+62' },
  { flag: '🇭🇰', code: '+852' },
  { flag: '🇦🇪', code: '+971' },
];

const STRIPE_STYLE = {
  style: {
    base: { fontSize: '14px', color: '#1a1a1a', fontFamily: 'system-ui, sans-serif', '::placeholder': { color: '#9ca3af' } },
    invalid: { color: '#e53e3e' },
  }
};

function CheckoutForm({ brand, selectedPlan, studioSlug, leadId, name, email, onSuccess }: {
  brand: string; selectedPlan: PublicPlan | null; studioSlug: string; leadId: string;
  name: string; email: string; onSuccess: (piId: string, plan: PublicPlan) => void;
}) {
  const stripe = useStripe(); const elements = useElements();
  const [loading, setLoading] = useState(false); const [error, setError] = useState('');

  async function handlePay(e: React.FormEvent) {
    e.preventDefault();
    if (!stripe || !elements || !selectedPlan) return;
    if (!name.trim()) { setError('Please enter your full name.'); return; }
    setLoading(true); setError('');
    if (selectedPlan.priceSgd === 0) { onSuccess('', selectedPlan); return; }
    try {
      const piRes = await fetch(`/api/v1/public/studios/${studioSlug}/payment-intent`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ planId: selectedPlan.id, leadId }),
      });
      if (!piRes.ok) { const b = await piRes.json().catch(() => null); throw new Error(b?.error || 'Failed to set up payment.'); }
      const { clientSecret } = await piRes.json();
      const card = elements.getElement(CardNumberElement);
      if (!card) throw new Error('Card not ready.');
      const { error: sErr, paymentIntent } = await stripe.confirmCardPayment(clientSecret, {
        payment_method: { card, billing_details: { name, email } },
      });
      if (sErr) throw new Error(sErr.message || 'Payment failed.');
      onSuccess(paymentIntent?.id ?? '', selectedPlan);
    } catch (err: any) { setError(err.message || 'Something went wrong.'); }
    finally { setLoading(false); }
  }

  const isFree = selectedPlan?.priceSgd === 0;
  const amount = selectedPlan ? (isFree ? 'Free' : `S$${(selectedPlan.priceSgd / 100).toFixed(2)}`) : '—';

  const stripeCls = 'border border-gray-300 rounded-lg px-3 py-2.5 bg-white focus-within:border-blue-500 focus-within:ring-1 focus-within:ring-blue-500 transition-colors';

  return (
    <form onSubmit={handlePay} className="space-y-3">
      {!isFree && (
        <>
          <div>
            <label className="block text-[11px] font-semibold text-gray-500 mb-1 uppercase tracking-wide">Card number</label>
            <div className={stripeCls}><CardNumberElement options={STRIPE_STYLE} /></div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-semibold text-gray-500 mb-1 uppercase tracking-wide">Expiry date</label>
              <div className={stripeCls}><CardExpiryElement options={STRIPE_STYLE} /></div>
            </div>
            <div>
              <label className="block text-[11px] font-semibold text-gray-500 mb-1 uppercase tracking-wide">Security code</label>
              <div className={stripeCls}><CardCvcElement options={STRIPE_STYLE} /></div>
            </div>
          </div>
        </>
      )}
      {error && (
        <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 px-3 py-2.5 text-sm text-red-700">
          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />{error}
        </div>
      )}
      <button type="submit" disabled={!selectedPlan || loading}
        className="w-full rounded-lg py-2.5 text-xs font-bold text-white disabled:opacity-40 transition-all flex items-center justify-center gap-1.5"
        style={{ background: loading ? '#6b7280' : brand }}>
        {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ShoppingCart className="h-3.5 w-3.5" />}
        {loading ? 'Processing…' : isFree ? 'Continue' : 'Complete Purchase'}
      </button>

      {/* Card logos */}
      <div className="grid grid-cols-3 gap-2 pt-1">
        {[
          { label: 'VISA', cls: 'italic font-black text-blue-900 text-sm' },
          { label: 'mastercard', cls: 'font-bold text-gray-700 text-[10px]' },
          { label: 'AMERICAN EXPRESS', cls: 'font-bold text-blue-700 text-[8px]' },
          { label: 'DISCOVER', cls: 'font-bold text-orange-600 text-[9px]' },
          { label: 'Apple Pay', cls: 'font-semibold text-gray-800 text-[10px]' },
          { label: 'G Pay', cls: 'font-bold text-gray-700 text-[10px]' },
        ].map(({ label, cls }) => (
          <div key={label} className="h-9 flex items-center justify-center rounded-lg border border-gray-200 bg-white shadow-sm">
            <span className={cls}>{label}</span>
          </div>
        ))}
      </div>
    </form>
  );
}

export function BookingPageClient({ studio, plans, leadId, studioSlug, campaignSlug, availabilitySlots, startAtSlot }: Props) {
  const brand = studio.brandColor || '#0ea5e9';
  const hasSlots = Object.values(availabilitySlots).some(s => s.length > 0);
  const [step, setStep] = useState<Step>(startAtSlot ? (hasSlots ? 'slot' : 'done') : 'checkout');
  const [selectedPlan, setSelectedPlan] = useState<PublicPlan | null>(plans[0] ?? null);
  const [receiptUrl, setReceiptUrl] = useState('');
  const [paidPlan, setPaidPlan] = useState<PublicPlan | null>(null);

  // Personal info
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [countryCode, setCountryCode] = useState('+65');
  const [phone, setPhone] = useState('');

  // Slot picker
  const [dates, setDates] = useState<{ dayName: string; dateStr: string; label: string }[]>([]);
  const [selectedDate, setSelectedDate] = useState('');
  const [selectedTime, setSelectedTime] = useState('');
  const [bookLoading, setBookLoading] = useState(false);
  const [bookError, setBookError] = useState('');
  const [submitLocked, setSubmitLocked] = useState(false);
  const [lockLeft, setLockLeft] = useState(0);

  const stripeRef = useRef<ReturnType<typeof loadStripe> | null>(null);
  if (!stripeRef.current && studio.stripePublishableKey) stripeRef.current = loadStripe(studio.stripePublishableKey);

  useEffect(() => {
    const wd = ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'], mo = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
    const list: typeof dates = [];
    for (let i = 0; i < 7; i++) {
      const d = new Date(); d.setDate(d.getDate() + i);
      if (d.getDay() === 0) continue;
      list.push({ dayName: wd[d.getDay()] ?? 'Mon', dateStr: `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`, label: `${mo[d.getMonth()] ?? 'Jan'} ${d.getDate()}` });
    }
    setDates(list); if (list[0]) setSelectedDate(list[0].dateStr);
  }, []);

  useEffect(() => {
    const last = localStorage.getItem(`last_submit_${leadId}`);
    if (last) { const e = Date.now() - parseInt(last, 10); if (e < 20000) { setSubmitLocked(true); setLockLeft(Math.ceil((20000 - e) / 1000)); } }
  }, [leadId]);

  useEffect(() => {
    if (!submitLocked || lockLeft <= 0) return;
    const t = setInterval(() => setLockLeft(p => { if (p <= 1) { setSubmitLocked(false); clearInterval(t); return 0; } return p - 1; }), 1000);
    return () => clearInterval(t);
  }, [submitLocked, lockLeft]);

  const timeSlots = selectedDate ? (availabilitySlots[new Date(selectedDate).toLocaleDateString('en-US', { weekday: 'long' })] || []) : [];

  async function handleBook() {
    if (!selectedDate || !selectedTime || bookLoading || submitLocked) return;
    setBookLoading(true); setBookError('');
    localStorage.setItem(`last_submit_${leadId}`, Date.now().toString());
    setSubmitLocked(true); setLockLeft(20);
    try {
      const res = await fetch(`/api/v1/public/leads/${encodeURIComponent(leadId)}/trial-slot`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ slot: `${selectedDate} ${selectedTime}` }) });
      if (!res.ok) throw new Error('Failed to book slot.');
      setStep('done');
    } catch (e: any) { setBookError(e.message); } finally { setBookLoading(false); }
  }

  function handlePaymentSuccess(piId: string, plan: PublicPlan) {
    setPaidPlan(plan);
    if (piId) fetch(`/api/v1/public/studios/${studioSlug}/payment-receipt/${piId}`).then(r => r.json()).then(d => { if (d.receiptUrl) setReceiptUrl(d.receiptUrl); }).catch(() => {});
    const isTrialOrFree = plan.priceSgd === 0 || plan.billingCycle === 'one_time' || plan.billingCycle === 'one-time';
    setStep(isTrialOrFree && hasSlots ? 'slot' : 'done');
  }

  const heroImage = studio.bookingHeroImageUrl;
  const heroVideo = studio.bookingHeroVideoUrl;
  const selPrice = selectedPlan ? (selectedPlan.priceSgd === 0 ? 'Free' : `S$${(selectedPlan.priceSgd / 100).toFixed(2)}`) : null;

  const StudioLogo = () => studio.logoUrl
    ? <img src={studio.logoUrl} alt={studio.name} className="h-8 w-8 rounded-lg object-cover" />
    : <div className="h-8 w-8 rounded-lg flex items-center justify-center font-bold text-xs text-white shrink-0" style={{ background: brand }}>{studio.name.slice(0,2).toUpperCase()}</div>;

  // ── DONE ─────────────────────────────────────────────────────────────────
  if (step === 'done') {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-md bg-white rounded-2xl shadow-lg p-10 text-center space-y-6">
          <div className="mx-auto h-20 w-20 rounded-full flex items-center justify-center" style={{ background: `${brand}18` }}>
            <CheckCircle2 className="h-10 w-10" style={{ color: brand }} />
          </div>
          <div>
            <h2 className="text-2xl font-bold text-gray-900">Payment Successful!</h2>
            {paidPlan && <p className="text-gray-500 mt-2 text-sm">{paidPlan.planName} · {paidPlan.billingCycle === 'monthly' ? 'Monthly' : paidPlan.billingCycle === 'yearly' ? 'Annual' : 'One-time'}</p>}
          </div>
          {selectedDate && selectedTime && (
            <div className="rounded-xl border border-gray-200 bg-gray-50 px-5 py-4 text-left space-y-2">
              <div className="flex items-center gap-2.5 text-sm text-gray-700"><Calendar className="h-4 w-4" style={{ color: brand }} />{selectedDate}</div>
              <div className="flex items-center gap-2.5 text-sm text-gray-700"><Clock className="h-4 w-4" style={{ color: brand }} />{selectedTime}</div>
            </div>
          )}
          {receiptUrl && (
            <a href={receiptUrl} target="_blank" rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg px-6 py-3 text-sm font-semibold text-white transition-all hover:opacity-90"
              style={{ background: brand }}>
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3M3 17V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" /></svg>
              Download Receipt
            </a>
          )}
          <p className="text-sm text-gray-400">We'll reach out on WhatsApp to confirm. 🚀</p>
          <p className="text-xs text-gray-300 pt-2">Powered by 1herosocial.ai</p>
        </div>
      </div>
    );
  }

  // ── SLOT PICKER ───────────────────────────────────────────────────────────
  if (step === 'slot') {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4 py-8">
        <div className="w-full max-w-lg bg-white rounded-2xl shadow-lg overflow-hidden">
          <div className="px-8 pt-6 pb-5 border-b border-gray-100 flex items-center gap-3">
            <StudioLogo />
            <div>
              <p className="font-bold text-gray-900 text-sm">{studio.name}</p>
              <p className="text-xs text-gray-400">Pick your trial session slot</p>
            </div>
          </div>
          <div className="px-8 py-6 space-y-5">
            {bookError && <div className="flex items-center gap-2 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"><AlertCircle className="h-4 w-4 shrink-0" />{bookError}</div>}
            <div className="space-y-2.5">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Select Date</p>
              <div className="grid grid-cols-3 gap-2 sm:grid-cols-6">
                {dates.map(d => { const isSel = selectedDate === d.dateStr; return (
                  <button key={d.dateStr} type="button" onClick={() => { setSelectedDate(d.dateStr); setSelectedTime(''); }}
                    className={`flex flex-col items-center rounded-xl border py-2.5 text-center transition-all ${isSel ? 'border-transparent text-white' : 'border-gray-200 text-gray-600 hover:border-gray-300'}`}
                    style={isSel ? { background: brand } : {}}>
                    <span className="text-[9px] font-bold uppercase tracking-wider opacity-70">{d.dayName}</span>
                    <span className="text-base font-bold mt-0.5 leading-none">{d.label.split(' ')[1]}</span>
                    <span className="text-[9px] font-medium opacity-60 mt-0.5">{d.label.split(' ')[0]}</span>
                  </button>
                ); })}
              </div>
            </div>
            <div className="space-y-2.5">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Select Time</p>
              {timeSlots.length === 0
                ? <p className="text-sm text-gray-400 bg-gray-50 rounded-xl p-4 text-center">No classes on this day — try another date.</p>
                : <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
                  {timeSlots.map((t: string) => { const isSel = selectedTime === t; return (
                    <button key={t} type="button" onClick={() => setSelectedTime(t)}
                      className={`rounded-xl border py-2.5 text-sm font-semibold transition-all ${isSel ? 'border-transparent text-white' : 'border-gray-200 text-gray-600 hover:border-gray-300'}`}
                      style={isSel ? { background: brand } : {}}>{t}
                    </button>
                  ); })}
                </div>}
            </div>
            <button type="button" disabled={!selectedDate || !selectedTime || bookLoading || submitLocked} onClick={handleBook}
              className="w-full rounded-lg py-3.5 text-sm font-bold text-white flex items-center justify-center gap-2 disabled:opacity-40 transition-all"
              style={{ background: brand }}>
              {bookLoading ? <><Loader2 className="h-4 w-4 animate-spin" />Booking…</> : submitLocked ? `Please wait (${lockLeft}s)` : 'Confirm Appointment'}
            </button>
          </div>
          <p className="text-center text-xs text-gray-300 pb-5">Powered by 1herosocial.ai</p>
        </div>
      </div>
    );
  }

  // ── CHECKOUT — two-column ─────────────────────────────────────────────────
  const fieldCls = 'w-full border border-gray-300 rounded-lg px-3 py-2 text-sm text-gray-800 placeholder:text-gray-400 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 bg-white transition-colors';

  return (
    <div className="min-h-screen bg-gray-100 flex flex-col">

      {/* Top bar */}
      <header className="bg-white border-b border-gray-200 px-6 py-3 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2.5">
          <StudioLogo />
          <span className="font-bold text-gray-800 text-sm">{studio.name}</span>
        </div>
        <div className="flex items-center gap-1.5 text-xs text-gray-400 font-medium">
          <Lock className="h-3.5 w-3.5" />SSL Secure
        </div>
      </header>

      {/* Two-column layout */}
      <div className="flex-1 flex flex-col lg:flex-row max-w-sm mx-auto lg:max-w-6xl w-full gap-4 px-3 py-4 lg:px-6 lg:py-8">

        {/* ── LEFT: Plan + Form ── */}
        <div className="flex-1 space-y-3 min-w-0 order-2 lg:order-1">

          {/* Plan dropdown — first thing on left */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 px-3 py-3 space-y-2">
            <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wide">Select Your Plan</h3>
            <select
              value={selectedPlan?.id ?? ''}
              onChange={e => setSelectedPlan(plans.find(p => p.id === e.target.value) ?? null)}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm text-gray-800 bg-white focus:outline-none focus:ring-1 focus:border-blue-500 appearance-none cursor-pointer">
              {plans.map(plan => {
                const price = plan.priceSgd === 0 ? 'Free' : `S$${(plan.priceSgd / 100).toFixed(2)}`;
                const cycle = plan.billingCycle === 'monthly' ? '/mo' : plan.billingCycle === 'yearly' ? '/yr' : '';
                return <option key={plan.id} value={plan.id}>{plan.planName} — {price}{cycle}</option>;
              })}
            </select>
            {selectedPlan && (
              <div className="rounded-lg bg-gray-50 border border-gray-100 px-3 py-2 space-y-1.5">
                {selectedPlan.features.length > 0 && (
                  <ul className="space-y-1">
                    {selectedPlan.features.map((f, i) => (
                      <li key={i} className="flex items-center gap-1.5 text-xs text-gray-500">
                        <span className="h-1 w-1 rounded-full shrink-0" style={{ background: brand }} />{f}
                      </li>
                    ))}
                  </ul>
                )}
                <div className="flex justify-between items-center pt-1.5 border-t border-gray-200">
                  <span className="text-xs font-bold text-gray-500 uppercase tracking-wide">Order Total</span>
                  <span className="text-sm font-black" style={{ color: brand }}>{selPrice}</span>
                </div>
              </div>
            )}
          </div>

          {/* Personal info card */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 px-3 py-3 space-y-2">
            <h2 className="text-xs font-bold text-gray-500 uppercase tracking-wide">Your Details</h2>
            <input value={name} onChange={e => setName(e.target.value)} placeholder="Full Name..." className={fieldCls} />
            <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="Email Address..." className={fieldCls} />
            <div className="flex">
              <select value={countryCode} onChange={e => setCountryCode(e.target.value)}
                className="border border-r-0 border-gray-300 rounded-l-lg px-2 py-2 text-sm text-gray-800 bg-white focus:outline-none focus:border-blue-500 shrink-0">
                {COUNTRY_CODES.map(c => (
                  <option key={c.code} value={c.code}>{c.flag} {c.code}</option>
                ))}
              </select>
              <input type="tel" value={phone} onChange={e => setPhone(e.target.value)} placeholder="Phone Number..."
                className="flex-1 border border-gray-300 rounded-r-lg px-3 py-2 text-sm text-gray-800 placeholder:text-gray-400 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 bg-white" />
            </div>
          </div>

          {/* Payment card */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 px-3 py-3">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-bold text-gray-500 uppercase tracking-wide">Payment</h2>
              <div className="flex items-center gap-1 text-xs text-gray-400"><Shield className="h-3 w-3" />256-bit SSL</div>
            </div>
            {stripeRef.current ? (
              <Elements stripe={stripeRef.current}>
                <CheckoutForm brand={brand} selectedPlan={selectedPlan} studioSlug={studioSlug} leadId={leadId} name={name} email={email} onSuccess={handlePaymentSuccess} />
              </Elements>
            ) : (
              <div className="space-y-3">
                {[1,2,3].map(i => <div key={i} className="h-12 rounded-lg bg-gray-100 animate-pulse" />)}
                <div className="h-14 rounded-lg bg-gray-200 animate-pulse" />
              </div>
            )}
          </div>

          <p className="text-center text-xs text-gray-300">Powered by 1herosocial.ai</p>
        </div>

        {/* ── RIGHT: Media + Plan selection ── */}
        <div className="lg:w-[420px] shrink-0 space-y-4 order-1 lg:order-2">

          {/* Hero video */}
          {heroVideo && (
            <div className="relative rounded-2xl overflow-hidden bg-gray-900" style={{ aspectRatio: '16/9' }}>
              <video autoPlay muted loop playsInline className="absolute inset-0 w-full h-full object-cover">
                <source src={heroVideo} type="video/mp4" />
              </video>
              <div className="absolute inset-0 bg-gradient-to-t from-black/40 to-transparent" />
              <div className="absolute bottom-3 left-3">
                <p className="font-bold text-white text-sm">{studio.name}</p>
              </div>
            </div>
          )}

          {/* Hero image */}
          {heroImage && (
            <div className="relative rounded-2xl overflow-hidden bg-gray-200" style={{ aspectRatio: '16/9' }}>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={heroImage} alt={studio.name} className="absolute inset-0 w-full h-full object-cover" />
              <div className="absolute inset-0 bg-gradient-to-t from-black/30 to-transparent" />
            </div>
          )}

          {/* Trust badges */}
          <div className="grid grid-cols-3 gap-2 text-center">
            {[['🔒', 'SSL Encrypted'], ['⚡', 'Instant Access'], ['💬', 'WhatsApp Confirm']].map(([icon, label]) => (
              <div key={label} className="bg-white rounded-xl border border-gray-200 py-2.5 px-2">
                <div className="text-lg leading-tight">{icon}</div>
                <p className="text-[10px] text-gray-400 font-medium mt-1 leading-tight">{label}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
