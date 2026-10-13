'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useRouter } from 'next/navigation';
import { UserPlus, X } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Label } from '@/components/ui/Label';
import { Input } from '@/components/ui/Input';
import type { Campaign } from '@/lib/types';
import { createLeadAction } from './actions';

interface AddLeadButtonProps {
  studioId: string;
  campaigns: Campaign[];
}

// Manually adding a single lead — a walk-in or phone inquiry the studio
// wants tracked, as opposed to a public form submission or bulk CSV import
// (see ImportLeadsButton). Same modal/action pattern as that component.
export function AddLeadButton({ studioId, campaigns }: AddLeadButtonProps) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [email, setEmail] = useState('');
  const [campaignId, setCampaignId] = useState(campaigns[0]?.id || '');
  const [fitnessPlan, setFitnessPlan] = useState('');
  const [sendGreeting, setSendGreeting] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    setMounted(true);
  }, []);

  function reset() {
    setName('');
    setPhone('');
    setEmail('');
    setFitnessPlan('');
    setSendGreeting(false);
    setErrors({});
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setErrors({});
    try {
      const res = await createLeadAction(studioId, {
        campaignId: campaignId || undefined,
        name: name.trim(),
        email: email.trim() || undefined,
        phone: phone.trim(),
        fitnessPlan: fitnessPlan.trim() || undefined,
        sendGreeting,
      });
      if (res.ok) {
        router.refresh();
        setIsOpen(false);
        reset();
      } else {
        setErrors(res.details ?? { _: res.error });
      }
    } finally {
      setLoading(false);
    }
  }

  const campaignOptions = campaigns.filter((c) => c.active !== false);

  return (
    <>
      <Button
        onClick={() => setIsOpen(true)}
        variant="ghost"
        size="sm"
        leftIcon={<UserPlus className="h-3.5 w-3.5" />}
        className="rounded-xl border border-white/20 bg-white/10 px-3 py-1.5 text-xs font-bold text-zinc-700 hover:bg-white/20 dark:text-zinc-200 dark:hover:bg-neutral-800/50 shadow-sm shrink-0"
      >
        Add Lead
      </Button>

      {isOpen && mounted && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div
            className="w-full max-w-md overflow-hidden rounded-[28px] border border-white/30 bg-white/80 p-6 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-zinc-900/80"
            style={{ boxShadow: '0 20px 50px rgba(0,0,0,0.3)' }}
          >
            <div className="flex items-center justify-between border-b border-zinc-200/50 pb-4 dark:border-zinc-800/50">
              <h2 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Add Lead</h2>
              <button
                onClick={() => {
                  setIsOpen(false);
                  reset();
                }}
                className="rounded-full p-1 text-zinc-400 hover:bg-zinc-200/50 dark:hover:bg-zinc-800/50"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleSubmit} className="mt-5 space-y-4">
              <div>
                <Label htmlFor="lead-name">Name</Label>
                <Input id="lead-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Jane Doe" invalid={!!errors.name} />
                {errors.name && <p className="mt-1 text-xs font-medium text-red-500">{errors.name}</p>}
              </div>

              <div>
                <Label htmlFor="lead-phone">Phone</Label>
                <Input id="lead-phone" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+65 9123 4567" invalid={!!errors.phone} />
                {errors.phone && <p className="mt-1 text-xs font-medium text-red-500">{errors.phone}</p>}
              </div>

              <div>
                <Label htmlFor="lead-email">Email (optional)</Label>
                <Input id="lead-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="jane@example.com" invalid={!!errors.email} />
                {errors.email && <p className="mt-1 text-xs font-medium text-red-500">{errors.email}</p>}
              </div>

              <div>
                <Label htmlFor="lead-campaign">Campaign</Label>
                {campaignOptions.length === 0 ? (
                  <div className="mt-1.5 flex items-center gap-2 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs font-semibold text-amber-700 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-400">
                    No active campaigns — create one first to attribute this lead to it.
                  </div>
                ) : (
                  <select
                    id="lead-campaign"
                    value={campaignId}
                    onChange={(e) => setCampaignId(e.target.value)}
                    className="mt-1.5 w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm font-semibold text-zinc-800 focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
                  >
                    {campaignOptions.map((c) => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                )}
              </div>

              <div>
                <Label htmlFor="lead-plan">Fitness plan (optional)</Label>
                <Input id="lead-plan" value={fitnessPlan} onChange={(e) => setFitnessPlan(e.target.value)} placeholder="Defaults to the campaign's first plan" />
              </div>

              <label className="flex items-start gap-2.5 rounded-2xl border border-zinc-200/50 bg-zinc-100/50 p-3.5 dark:border-zinc-800/50 dark:bg-zinc-800/30">
                <input
                  type="checkbox"
                  checked={sendGreeting}
                  onChange={(e) => setSendGreeting(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-zinc-300"
                />
                <span className="text-xs text-zinc-600 dark:text-zinc-300">
                  Send the automated WhatsApp greeting now. Off by default — the lead is added without messaging them.
                </span>
              </label>

              {errors._ && <p className="text-xs font-medium text-red-500">{errors._}</p>}

              <div className="flex justify-end gap-2 pt-2">
                <Button type="button" variant="ghost" size="sm" onClick={() => { setIsOpen(false); reset(); }}>
                  Cancel
                </Button>
                <Button type="submit" size="sm" disabled={loading || !phone.trim() || !name.trim()}>
                  {loading ? 'Adding…' : 'Add Lead'}
                </Button>
              </div>
            </form>
          </div>
        </div>,
        document.body,
      )}
    </>
  );
}
