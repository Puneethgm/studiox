'use client';

import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { loadStripe } from '@stripe/stripe-js';
import { Elements, CardNumberElement, CardExpiryElement, CardCvcElement, useStripe, useElements } from '@stripe/react-stripe-js';
import { CheckCircle2, AlertCircle, Loader2, Eye } from 'lucide-react';
import type {
  PageBlock, PageBackground, TextBlockContent, ImageBlockContent, VideoBlockContent, FieldBlockContent,
  AmountBlockContent, PayButtonBlockContent,
} from '@/lib/types';
import { FONT_FAMILY_STACKS } from '@/lib/types';
import { defaultTrialPageBlocks, defaultTrialPageBackground, computeCanvasHeight, CANVAS_WIDTH } from '@/lib/trialPageDefaults';

interface LeadInfo { leadName: string; studioName: string; alreadyPurchased: boolean; }
interface StudioInfo { name: string; brandColor: string; logoUrl: string; trialAmountSgd?: number; stripePublishableKey?: string; }

const STRIPE_STYLE = {
  style: {
    base: { fontSize: '14px', color: '#1a1a1a', fontFamily: 'system-ui, sans-serif', '::placeholder': { color: '#9ca3af' } },
    invalid: { color: '#e53e3e' },
  },
};

const fieldCls = 'w-full rounded-lg border border-gray-300 px-3 py-2 text-sm placeholder:text-gray-400 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500';

function FieldInput({ label, value, onChange, type = 'text', required, content }: {
  label: string; value: string; onChange: (v: string) => void; type?: string; required?: boolean; content: FieldBlockContent;
}) {
  return (
    <div className="flex h-full flex-col justify-center gap-1">
      <label className="text-[10px] font-bold uppercase tracking-wide" style={{ color: content.labelColor ?? '#9ca3af' }}>{label}</label>
      <input
        type={type}
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={fieldCls}
        style={{ backgroundColor: content.backgroundColor ?? '#ffffff', color: content.textColor ?? '#1f2937' }}
      />
    </div>
  );
}

function GenderInput({ label, value, onChange, content }: { label: string; value: string; onChange: (v: string) => void; content: FieldBlockContent }) {
  return (
    <div className="flex h-full flex-col justify-center gap-1">
      <label className="text-[10px] font-bold uppercase tracking-wide" style={{ color: content.labelColor ?? '#9ca3af' }}>{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={fieldCls}
        style={{ backgroundColor: content.backgroundColor ?? '#ffffff', color: content.textColor ?? '#1f2937' }}
      >
        <option value="">Prefer not to say</option>
        <option value="female">Female</option>
        <option value="male">Male</option>
        <option value="other">Other</option>
      </select>
    </div>
  );
}

/** Scales the fixed-width design canvas down to fit any viewport narrower
 * than it (phones under ~390px, or a desktop window resized small) — the
 * canvas is designed at one fixed coordinate system (CANVAS_WIDTH) so every
 * block's x/y/width/height stays proportionally correct at any screen size,
 * rather than trying to reflow individual absolutely-positioned blocks. */
function useResponsiveScale(designWidth: number) {
  const [scale, setScale] = useState(1);
  useEffect(() => {
    function update() {
      const available = window.innerWidth - 32; // page padding
      setScale(Math.min(1, available / designWidth));
    }
    update();
    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }, [designWidth]);
  return scale;
}

function CanvasForm({ blocks, background, leadInfo, amount, leadId, studioSlug, preview, standalone, onSuccess }: {
  blocks: PageBlock[]; background: PageBackground; leadInfo: LeadInfo; amount: number | null; leadId?: string; studioSlug?: string; preview?: boolean; standalone?: boolean; onSuccess: () => void;
}) {
  const stripe = useStripe();
  const elements = useElements();
  const scale = useResponsiveScale(CANVAS_WIDTH);
  const [fullName, setFullName] = useState(leadInfo.leadName || '');
  const [gender, setGender] = useState('');
  const [dateOfBirth, setDateOfBirth] = useState('');
  const [phone, setPhone] = useState('');
  const [showPhoneModal, setShowPhoneModal] = useState(false);
  const [phoneError, setPhoneError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function onPayClick() {
    if (submitting) return;
    if (preview) {
      setError('This is a preview — payments are disabled here. Open the real link from a WhatsApp message to pay.');
      return;
    }
    if (!fullName.trim()) {
      setError('Please enter your full name.');
      return;
    }
    if (!standalone && !leadId) {
      setError('This is a preview — payments are disabled here. Open the real link from a WhatsApp message to pay.');
      return;
    }
    // Static/no-lead links don't collect a phone number up front — ask for
    // it right after "Confirm" is clicked, then proceed to payment.
    if (standalone) {
      setError(null);
      setShowPhoneModal(true);
      return;
    }
    handlePay();
  }

  function confirmPhoneAndPay() {
    if (!phone.trim()) {
      setPhoneError('Please enter your phone number.');
      return;
    }
    setPhoneError(null);
    setShowPhoneModal(false);
    handlePay();
  }

  async function handlePay() {
    if (submitting) return;
    if (!stripe || !elements) {
      setError('Payment form is still loading — please try again in a moment.');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      let activeLeadId = leadId;
      if (standalone) {
        const signupRes = await fetch(`/api/v1/public/studios/${encodeURIComponent(studioSlug || '')}/trial-signup`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ fullName: fullName.trim(), phone: phone.trim(), gender, dateOfBirth }),
        });
        if (!signupRes.ok) {
          const b = await signupRes.json().catch(() => null);
          throw new Error(b?.error || 'Failed to sign up — please check your details.');
        }
        const signupData = await signupRes.json();
        activeLeadId = signupData.leadId;
      } else {
        await fetch(`/api/v1/public/leads/${encodeURIComponent(activeLeadId!)}/trial-checkout`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ fullName: fullName.trim(), gender, dateOfBirth }),
        });
      }
      const piRes = await fetch(`/api/v1/public/leads/${encodeURIComponent(activeLeadId!)}/trial-payment-intent`, { method: 'POST' });
      if (!piRes.ok) {
        const b = await piRes.json().catch(() => null);
        throw new Error(b?.error || 'Failed to set up payment.');
      }
      const { clientSecret } = await piRes.json();
      const card = elements.getElement(CardNumberElement);
      if (!card) throw new Error('Card not ready.');
      const { error: sErr } = await stripe.confirmCardPayment(clientSecret, {
        payment_method: { card, billing_details: { name: fullName.trim() } },
      });
      if (sErr) throw new Error(sErr.message || 'Payment failed.');
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong.');
    } finally {
      setSubmitting(false);
    }
  }

  const canvasHeight = computeCanvasHeight(blocks);
  const canvasStyle: React.CSSProperties = {
    width: CANVAS_WIDTH,
    height: canvasHeight,
    backgroundColor: background.color,
    backgroundImage: background.imageUrl ? `url(${background.imageUrl})` : undefined,
    backgroundSize: 'cover',
    backgroundPosition: 'center',
  };

  return (
    <div className="mx-auto" style={{ width: CANVAS_WIDTH * scale }}>
      {preview && (
        <div className="mb-3 flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-700">
          <Eye className="h-3.5 w-3.5 shrink-0" />
          Preview mode — this is exactly how your page looks. Payments are disabled here.
        </div>
      )}
      <div className="relative overflow-hidden" style={{ width: CANVAS_WIDTH * scale, height: canvasHeight * scale }}>
        <div
          className="absolute left-0 top-0 origin-top-left overflow-hidden rounded-2xl"
          style={{ ...canvasStyle, transform: `scale(${scale})` }}
        >
        {blocks.map((block) => (
          <div
            key={block.id}
            className="absolute"
            style={{ left: block.x, top: block.y, width: block.width, height: block.height, zIndex: block.zIndex }}
          >
            {block.type === 'text' && (() => {
              const c = block.content as TextBlockContent;
              return (
                <div style={{
                  fontSize: c.fontSize,
                  color: c.color,
                  fontWeight: c.weight === 'bold' ? 700 : 400,
                  fontStyle: c.italic ? 'italic' : 'normal',
                  textDecoration: c.underline ? 'underline' : 'none',
                  textAlign: c.align ?? 'left',
                  fontFamily: FONT_FAMILY_STACKS[c.fontFamily] ?? FONT_FAMILY_STACKS.system,
                }}>
                  {c.text}
                </div>
              );
            })()}

            {block.type === 'image' && (() => {
              const c = block.content as ImageBlockContent;
              return c.url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={c.url} alt="" className="h-full w-full rounded-lg object-cover" />
              ) : null;
            })()}

            {block.type === 'video' && (() => {
              const c = block.content as VideoBlockContent;
              return c.url ? (
                <video src={c.url} autoPlay muted loop playsInline className="h-full w-full rounded-lg object-cover" />
              ) : null;
            })()}

            {block.type === 'name_field' && (
              <FieldInput content={block.content as FieldBlockContent} label={(block.content as FieldBlockContent).label} value={fullName} onChange={setFullName} required />
            )}
            {block.type === 'gender_field' && (
              <GenderInput content={block.content as FieldBlockContent} label={(block.content as FieldBlockContent).label} value={gender} onChange={setGender} />
            )}
            {block.type === 'dob_field' && (
              <FieldInput content={block.content as FieldBlockContent} label={(block.content as FieldBlockContent).label} value={dateOfBirth} onChange={setDateOfBirth} type="date" />
            )}

            {block.type === 'amount_display' && (() => {
              const c = block.content as AmountBlockContent;
              return (
                <div className="flex h-full items-center justify-between rounded-lg border border-gray-100 px-3" style={{ backgroundColor: c.backgroundColor ?? '#f9fafb' }}>
                  <span className="text-xs font-semibold" style={{ color: c.labelColor ?? '#6b7280' }}>{c.label}</span>
                  <span className="text-sm font-black" style={{ color: c.textColor ?? '#111827' }}>{amount != null ? `S$${(amount / 100).toFixed(2)}` : '—'}</span>
                </div>
              );
            })()}

            {block.type === 'card_fields' && (
              <div className="space-y-2">
                <div className="rounded-lg border border-gray-300 bg-white px-3 py-2.5">
                  <CardNumberElement options={STRIPE_STYLE} />
                </div>
                <div className="flex gap-2">
                  <div className="flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2.5">
                    <CardExpiryElement options={STRIPE_STYLE} />
                  </div>
                  <div className="flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2.5">
                    <CardCvcElement options={STRIPE_STYLE} />
                  </div>
                </div>
              </div>
            )}

            {block.type === 'pay_button' && (() => {
              const c = block.content as PayButtonBlockContent;
              return (
                <button
                  type="button"
                  onClick={onPayClick}
                  disabled={submitting}
                  className="flex h-full w-full items-center justify-center gap-2 rounded-lg text-sm font-bold text-white disabled:opacity-60 transition-opacity"
                  style={{ background: c.color }}
                >
                  {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
                  {submitting ? 'Processing…' : c.label}
                </button>
              );
            })()}
          </div>
        ))}
        </div>
      </div>
      {error && (
        <div className="mt-3 flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 px-3 py-2.5 text-sm text-red-700">
          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />{error}
        </div>
      )}
      <p className="mt-4 text-center text-xs text-gray-300">Powered by 1herosocial.ai</p>

      {showPhoneModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-6" onClick={() => setShowPhoneModal(false)}>
          <div className="w-full max-w-xs rounded-2xl bg-white p-5 shadow-2xl" onClick={(e) => e.stopPropagation()}>
            <h2 className="text-sm font-black text-gray-900">One last thing</h2>
            <p className="mt-1 text-xs text-gray-400">What&apos;s your phone number? We&apos;ll use it to confirm your trial on WhatsApp.</p>
            <input
              type="tel"
              autoFocus
              placeholder="+65 1234 5678"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && confirmPhoneAndPay()}
              className={`${fieldCls} mt-3`}
            />
            {phoneError && <p className="mt-1.5 text-xs font-semibold text-red-600">{phoneError}</p>}
            <button
              type="button"
              onClick={confirmPhoneAndPay}
              className="mt-3 flex h-10 w-full items-center justify-center rounded-lg text-sm font-bold text-white"
              style={{ background: '#7c3aed' }}
            >
              Confirm
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export function TrialPaymentPage({ leadId, preview, standalone, studioSlug: studioSlugProp }: { leadId?: string; preview?: boolean; standalone?: boolean; studioSlug?: string }) {
  const searchParams = useSearchParams();
  const studioSlug = studioSlugProp || searchParams.get('studio') || '';

  const [leadInfo, setLeadInfo] = useState<LeadInfo | null>(null);
  const [studio, setStudio] = useState<StudioInfo | null>(null);
  const [amount, setAmount] = useState<number | null>(null);
  const [blocks, setBlocks] = useState<PageBlock[] | null>(null);
  const [background, setBackground] = useState<PageBackground>(defaultTrialPageBackground());
  const [loadError, setLoadError] = useState(false);
  const [done, setDone] = useState(false);
  const stripeRef = useRef<ReturnType<typeof loadStripe> | null>(null);

  useEffect(() => {
    (async () => {
      try {
        if (preview) {
          // No real lead to preview against — synthesize placeholder info so
          // the page renders exactly as designed, just non-functional.
          setLeadInfo({ leadName: 'Preview User', studioName: '', alreadyPurchased: false });
        } else if (standalone || !leadId) {
          // Static, no-lead-attached signup link — a lead doesn't exist yet,
          // it's created on submit.
          setLeadInfo({ leadName: '', studioName: '', alreadyPurchased: false });
        } else {
          const leadRes = await fetch(`/api/v1/public/leads/${encodeURIComponent(leadId)}/trial-checkout`);
          if (!leadRes.ok) {
            setLoadError(true);
            return;
          }
          setLeadInfo(await leadRes.json());
        }

        if (studioSlug) {
          const [studioRes, layoutRes] = await Promise.all([
            fetch(`/api/v1/public/studios/${encodeURIComponent(studioSlug)}`),
            fetch(`/api/v1/public/studios/${encodeURIComponent(studioSlug)}/trial-page-layout`),
          ]);
          if (studioRes.ok) {
            const s: StudioInfo = await studioRes.json();
            setStudio(s);
            setAmount(s.trialAmountSgd && s.trialAmountSgd > 0 ? s.trialAmountSgd : 2500);
            if (s.stripePublishableKey && !stripeRef.current) {
              stripeRef.current = loadStripe(s.stripePublishableKey);
            }
          }
          if (layoutRes.ok) {
            const data = await layoutRes.json();
            setBlocks(data.blocks && data.blocks.length > 0 ? data.blocks : defaultTrialPageBlocks());
            setBackground(data.background ?? defaultTrialPageBackground());
          } else {
            setBlocks(defaultTrialPageBlocks());
          }
        } else {
          setBlocks(defaultTrialPageBlocks());
        }
      } catch {
        setLoadError(true);
      }
    })();
  }, [leadId, studioSlug, preview]);

  const brand = studio?.brandColor || '#7c3aed';

  return (
    <div className="min-h-screen bg-gray-50 sm:bg-gradient-to-br sm:from-slate-100 sm:via-gray-50 sm:to-slate-100 flex items-center justify-center p-6">
      <div className="w-full max-w-md">
        {loadError ? (
          <div className="rounded-2xl border border-gray-200 bg-white p-8 text-center space-y-2 shadow-sm">
            <p className="text-sm font-bold text-gray-900">This link isn&apos;t valid.</p>
            <p className="text-xs text-gray-400">Please contact the studio for a new booking link.</p>
          </div>
        ) : done ? (
          <div className="rounded-2xl border border-gray-200 bg-white p-10 text-center space-y-4 shadow-sm">
            <div className="mx-auto h-16 w-16 rounded-full flex items-center justify-center" style={{ background: `${brand}18` }}>
              <CheckCircle2 className="h-8 w-8" style={{ color: brand }} />
            </div>
            <h1 className="text-xl font-black text-gray-900">Payment Successful!</h1>
            <p className="text-sm text-gray-500">
              Your trial at <span className="font-bold text-gray-900">{studio?.name || leadInfo?.studioName}</span> is confirmed. We&apos;ll reach out on WhatsApp shortly. 🚀
            </p>
          </div>
        ) : !leadInfo || !blocks ? (
          <div className="flex justify-center py-16">
            <div className="h-8 w-8 rounded-full border-2 border-violet-500 border-t-transparent animate-spin" />
          </div>
        ) : leadInfo.alreadyPurchased ? (
          <div className="rounded-2xl border border-gray-200 bg-white p-10 text-center space-y-3 shadow-sm">
            <div className="mx-auto h-16 w-16 rounded-full flex items-center justify-center bg-emerald-50 border border-emerald-200">
              <CheckCircle2 className="h-8 w-8 text-emerald-500" />
            </div>
            <h1 className="text-xl font-black text-gray-900">You&apos;re all set!</h1>
            <p className="text-sm text-gray-500">
              Your trial at <span className="font-bold text-gray-900">{leadInfo.studioName}</span> is already confirmed.
            </p>
          </div>
        ) : stripeRef.current ? (
          <div className="sm:rounded-[28px] sm:bg-white sm:p-6 sm:shadow-2xl sm:shadow-black/5 sm:ring-1 sm:ring-black/5">
            <Elements stripe={stripeRef.current}>
              <CanvasForm blocks={blocks} background={background} leadInfo={leadInfo} amount={amount} leadId={leadId} studioSlug={studioSlug} preview={preview} standalone={standalone} onSuccess={() => setDone(true)} />
            </Elements>
          </div>
        ) : (
          <div className="mx-auto w-full space-y-3" style={{ maxWidth: CANVAS_WIDTH }}>
            {[1, 2, 3].map((i) => <div key={i} className="h-14 rounded-lg bg-gray-100 animate-pulse" />)}
          </div>
        )}
      </div>
    </div>
  );
}
