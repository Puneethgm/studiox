'use client';

import { useState } from 'react';
import { Check, MessageSquareText, Link2 } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import type { Studio } from '@/lib/types';
import { updateStudioSettings } from '../settings/actions';

const PLACEHOLDERS = [
  { token: '{{lead_first_name}}', desc: "the lead's first name" },
  { token: '{{studio_name}}', desc: 'this studio’s name' },
  { token: '{{amount}}', desc: 'the amount paid, e.g. "61.00 SGD"' },
  { token: '{{receipt_url}}', desc: 'link to the Stripe receipt' },
];

const DEFAULT_TRIAL_MESSAGE =
  '🎉 Hi {{lead_first_name}}! Thank you for booking your Trial at *{{studio_name}}*!\n\n' +
  'Your payment of *{{amount}}* was received successfully. We can’t wait to see you! 💪\n\n' +
  'Your session is confirmed. Please arrive 10 minutes early.\n\n' +
  '📄 *Your Receipt:* {{receipt_url}}\n\n' +
  'See you soon! — The {{studio_name}} Team';

const DEFAULT_MEMBERSHIP_MESSAGE =
  '🎉 Hi {{lead_first_name}}! Welcome to *{{studio_name}}*!\n\n' +
  'Your membership subscription of *{{amount}}* was received successfully. We are excited to have you on board! 💪\n\n' +
  '📄 *Your Receipt:* {{receipt_url}}\n\n' +
  'See you soon! — The {{studio_name}} Team';

function PlaceholderHints() {
  return (
    <div className="mt-2 flex flex-wrap gap-1.5">
      {PLACEHOLDERS.map((p) => (
        <span
          key={p.token}
          title={p.desc}
          className="rounded-full bg-brand-500/10 px-2 py-0.5 font-mono text-[10px] font-bold text-brand-600 dark:text-brand-400"
        >
          {p.token}
        </span>
      ))}
    </div>
  );
}

export function TemplatesClient({ studio }: { studio: Studio }) {
  const [trialMessage, setTrialMessage] = useState(studio.trialConfirmationMessage || DEFAULT_TRIAL_MESSAGE);
  const [membershipMessage, setMembershipMessage] = useState(studio.membershipConfirmationMessage || DEFAULT_MEMBERSHIP_MESSAGE);
  const [trialMembershipId, setTrialMembershipId] = useState(studio.trialGlofoxMembershipId || '');
  const [trialPlanCode, setTrialPlanCode] = useState(studio.trialGlofoxPlanCode || '');
  const [membershipMembershipId, setMembershipMembershipId] = useState(studio.membershipGlofoxMembershipId || '');
  const [membershipPlanCode, setMembershipPlanCode] = useState(studio.membershipGlofoxPlanCode || '');
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState<string | null>(null);

  async function handleSave() {
    setSaving(true);
    setToast(null);
    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        trialConfirmationMessage: trialMessage,
        membershipConfirmationMessage: membershipMessage,
        trialGlofoxMembershipId: trialMembershipId,
        trialGlofoxPlanCode: trialPlanCode,
        membershipGlofoxMembershipId: membershipMembershipId,
        membershipGlofoxPlanCode: membershipPlanCode,
      });
      if (result.ok) {
        setToast('Templates saved.');
      } else {
        setToast(`Failed to save: ${result.error}`);
      }
    } finally {
      setSaving(false);
      setTimeout(() => setToast(null), 4000);
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex items-center gap-2 mb-1">
          <MessageSquareText className="h-4 w-4 text-brand-500" />
          <h3 className="text-sm font-black text-zinc-950 dark:text-white">Trial Payment Confirmation</h3>
        </div>
        <p className="text-[11px] text-zinc-400 mb-3 leading-relaxed">
          Sent automatically right after a lead pays for a trial session. Leave blank to use the default below.
        </p>
        <textarea
          value={trialMessage}
          onChange={(e) => setTrialMessage(e.target.value)}
          placeholder={DEFAULT_TRIAL_MESSAGE}
          rows={8}
          className="w-full rounded-xl border border-zinc-200/50 bg-white/50 px-3 py-2.5 text-sm font-medium text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 dark:text-zinc-100 font-mono whitespace-pre-wrap"
        />
        <PlaceholderHints />
      </Card>

      <Card>
        <div className="flex items-center gap-2 mb-1">
          <MessageSquareText className="h-4 w-4 text-brand-500" />
          <h3 className="text-sm font-black text-zinc-950 dark:text-white">Membership Payment Confirmation</h3>
        </div>
        <p className="text-[11px] text-zinc-400 mb-3 leading-relaxed">
          Sent automatically right after a lead subscribes to a membership plan. Leave blank to use the default below.
        </p>
        <textarea
          value={membershipMessage}
          onChange={(e) => setMembershipMessage(e.target.value)}
          placeholder={DEFAULT_MEMBERSHIP_MESSAGE}
          rows={8}
          className="w-full rounded-xl border border-zinc-200/50 bg-white/50 px-3 py-2.5 text-sm font-medium text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 dark:text-zinc-100 font-mono whitespace-pre-wrap"
        />
        <PlaceholderHints />
      </Card>

      <Card>
        <div className="flex items-center gap-2 mb-1">
          <Link2 className="h-4 w-4 text-brand-500" />
          <h3 className="text-sm font-black text-zinc-950 dark:text-white">Glofox Membership Mapping</h3>
        </div>
        <p className="text-[11px] text-zinc-400 mb-4 leading-relaxed">
          Optional. When set, a real trial/membership payment also creates an actual credit-pack/membership purchase in Glofox (not just a bare lead record) —
          look these up once via Glofox&rsquo;s own dashboard (<code className="font-mono">GET /2.0/memberships</code>) and enter them here. Leave blank to skip this step.
        </p>
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className="mb-1 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
              Trial: Glofox Membership ID
            </label>
            <input
              value={trialMembershipId}
              onChange={(e) => setTrialMembershipId(e.target.value)}
              placeholder="(optional)"
              className="w-full rounded-xl border border-zinc-200/50 bg-white/50 px-3 py-2 text-sm font-mono text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 dark:text-zinc-100"
            />
          </div>
          <div>
            <label className="mb-1 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
              Trial: Glofox Plan Code
            </label>
            <input
              value={trialPlanCode}
              onChange={(e) => setTrialPlanCode(e.target.value)}
              placeholder="(optional)"
              className="w-full rounded-xl border border-zinc-200/50 bg-white/50 px-3 py-2 text-sm font-mono text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 dark:text-zinc-100"
            />
          </div>
          <div>
            <label className="mb-1 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
              Membership: Glofox Membership ID
            </label>
            <input
              value={membershipMembershipId}
              onChange={(e) => setMembershipMembershipId(e.target.value)}
              placeholder="(optional)"
              className="w-full rounded-xl border border-zinc-200/50 bg-white/50 px-3 py-2 text-sm font-mono text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 dark:text-zinc-100"
            />
          </div>
          <div>
            <label className="mb-1 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
              Membership: Glofox Plan Code
            </label>
            <input
              value={membershipPlanCode}
              onChange={(e) => setMembershipPlanCode(e.target.value)}
              placeholder="(optional)"
              className="w-full rounded-xl border border-zinc-200/50 bg-white/50 px-3 py-2 text-sm font-mono text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 dark:text-zinc-100"
            />
          </div>
        </div>
      </Card>

      <div className="flex items-center gap-3">
        <Button onClick={handleSave} loading={saving} leftIcon={<Check className="h-4 w-4" />}>
          Save Templates
        </Button>
        {toast && (
          <span className="text-xs font-bold text-zinc-500 dark:text-zinc-400">{toast}</span>
        )}
      </div>
    </div>
  );
}
