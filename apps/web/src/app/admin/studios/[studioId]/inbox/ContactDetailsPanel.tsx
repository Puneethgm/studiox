'use client';

import { useEffect, useState, useCallback } from 'react';
import { User, Loader2, BellOff, Plus, X, Globe, Mail, Phone, Shield, FileText, Check, Search } from 'lucide-react';
import { cn } from '@/lib/cn';
import { brandInitials } from '@/lib/color';
import { ApiError, api } from '@/lib/api';
import type { Conversation, Lead } from '@/lib/types';
import { LEAD_STATUS_LABELS } from '@/lib/types';

function getLeadStatusStyles(status: string): string {
  switch (status) {
    case 'new':
      return 'bg-blue-500/15 text-blue-600 dark:bg-blue-500/25 dark:text-blue-400';
    case 'contacted':
      return 'bg-violet-500/15 text-violet-600 dark:bg-violet-500/25 dark:text-violet-400';
    case 'trial_booked':
      return 'bg-emerald-500/15 text-emerald-600 dark:bg-emerald-500/25 dark:text-emerald-400';
    case 'member':
      return 'bg-amber-500/15 text-amber-600 dark:bg-amber-500/25 dark:text-amber-400';
    case 'dropped':
      return 'bg-rose-500/15 text-rose-600 dark:bg-rose-500/25 dark:text-rose-400';
    case 'paused':
      return 'bg-zinc-500/15 text-zinc-600 dark:bg-zinc-500/25 dark:text-zinc-400';
    default:
      return 'bg-zinc-100 text-zinc-600 dark:bg-white/5 dark:text-zinc-400';
  }
}

function getLeadStatusLabel(status: string): string {
  switch (status) {
    case 'new': return 'New';
    case 'contacted': return 'Connected';
    case 'trial_booked': return 'Trial';
    case 'member': return 'Member';
    case 'dropped': return 'Dropped';
    case 'paused': return 'Paused';
    default: return status;
  }
}

export function ContactDetailsPanel({
  studioId,
  conversation,
  onClose,
  onLeadUpdated,
}: {
  studioId: string;
  conversation: Conversation;
  onClose?: () => void;
  onLeadUpdated?: (lead: Lead) => void;
}) {
  const [lead, setLead] = useState<Lead | null>(null);
  const [loading, setLoading] = useState(false);
  const [dndSaving, setDndSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Tabbed view in contact panel (All fields, DND, Actions)
  const [activeSubTab, setActiveSubTab] = useState<'fields' | 'dnd' | 'actions'>('fields');

  // Accordion section & search query
  const [expandedSection, setExpandedSection] = useState<'contact' | 'social' | 'hapana' | null>('contact');
  const [searchQuery, setSearchQuery] = useState('');
  
  // Custom tag management
  const [customTags, setCustomTags] = useState<string[]>([]);
  const [isAddingTag, setIsAddingTag] = useState(false);
  const [newTagInput, setNewTagInput] = useState('');

  // Team users and Follower state
  const [users, setUsers] = useState<{ id: string; email: string; role: string }[]>([]);
  const [follower, setFollower] = useState('');

  const leadId = conversation.leadId;

  // Load Lead details
  useEffect(() => {
    setLead(null);
    setError(null);
    if (!leadId) return;
    setLoading(true);
    api<Lead>(`/api/v1/studios/${studioId}/leads/${leadId}`)
      .then((res) => {
        setLead(res);
        const storedFollower = localStorage.getItem(`projectx_lead_follower_${res.id}`);
        setFollower(storedFollower || '');
      })
      .catch((err: any) => setError(err?.message || 'Failed to load contact.'))
      .finally(() => setLoading(false));
  }, [studioId, leadId]);

  // Load studio users
  useEffect(() => {
    if (!studioId) return;
    api<{ users: { id: string; email: string; role: string }[] }>(`/api/v1/studios/${studioId}/users`)
      .then((res) => setUsers(res.users))
      .catch((err) => console.error('Failed to load studio users:', err));
  }, [studioId]);

  // Load custom tags from local storage when lead changes
  useEffect(() => {
    if (leadId) {
      try {
        const stored = localStorage.getItem(`projectx_lead_tags_${leadId}`);
        if (stored) {
          setCustomTags(JSON.parse(stored));
        } else {
          setCustomTags([]);
        }
      } catch (e) {
        console.error('Failed to load custom tags', e);
      }
    } else {
      setCustomTags([]);
    }
  }, [leadId]);

  // Save custom tags
  const saveCustomTags = (tags: string[]) => {
    setCustomTags(tags);
    if (leadId) {
      try {
        localStorage.setItem(`projectx_lead_tags_${leadId}`, JSON.stringify(tags));
      } catch (e) {
        console.error('Failed to save custom tags', e);
      }
    }
  };

  const handleAddTag = (e: React.FormEvent) => {
    e.preventDefault();
    const tag = newTagInput.trim();
    if (tag && !customTags.includes(tag)) {
      const nextTags = [...customTags, tag];
      saveCustomTags(nextTags);
    }
    setNewTagInput('');
    setIsAddingTag(false);
  };

  const handleRemoveTag = (tagToRemove: string) => {
    const nextTags = customTags.filter(t => t !== tagToRemove);
    saveCustomTags(nextTags);
  };

  async function updateLeadField(fields: Partial<Lead>) {
    if (!lead) return;
    setError(null);
    const originalLead = { ...lead };
    const updatedLead = { ...lead, ...fields };
    setLead(updatedLead);
    try {
      const updated = await api<Lead>(`/api/v1/studios/${studioId}/leads/${lead.id}`, {
        method: 'PATCH',
        json: fields,
      });
      setLead(updated);
      if (onLeadUpdated) {
        onLeadUpdated(updated);
      }
    } catch (err: any) {
      setLead(originalLead);
      setError(err?.message || 'Failed to update contact details.');
    }
  }

  async function toggleDND() {
    if (!lead || dndSaving) return;
    const nextEnabled = !lead.dndEnabled;
    setDndSaving(true);
    setError(null);
    setLead({ ...lead, dndEnabled: nextEnabled });
    try {
      const updated = await api<Lead>(`/api/v1/studios/${studioId}/leads/${lead.id}/dnd`, {
        method: 'PATCH',
        json: { enabled: nextEnabled },
      });
      setLead(updated);
    } catch (err: any) {
      setLead(lead);
      setError(err?.message || 'Failed to update Do Not Disturb.');
    } finally {
      setDndSaving(false);
    }
  }

  // Generate system-derived tags from lead fields
  const systemTags = lead
    ? [
        { label: getLeadStatusLabel(lead.status), style: getLeadStatusStyles(lead.status) },
        lead.fitnessPlan ? { label: lead.fitnessPlan, style: 'bg-emerald-500/15 text-emerald-600 dark:bg-emerald-500/25 dark:text-emerald-400' } : null,
        lead.source ? { label: `Source: ${lead.source}`, style: 'bg-blue-500/15 text-blue-600 dark:bg-blue-500/25 dark:text-blue-400' } : null,
        lead.referrer ? { label: `Referrer: ${lead.referrer}`, style: 'bg-amber-500/15 text-amber-600 dark:bg-amber-500/25 dark:text-amber-400' } : null,
      ].filter(Boolean) as { label: string; style: string }[]
    : [];

  const totalTagsCount = systemTags.length + customTags.length;

  return (
    <aside className="hidden w-80 shrink-0 flex-col border-l border-zinc-200 lg:flex bg-white/5 dark:border-white/5 dark:bg-neutral-950/10">
      <div className="flex h-14 shrink-0 items-center justify-between border-b border-zinc-100 px-5 dark:border-white/5">
        <h2 className="text-sm font-black uppercase tracking-wider text-zinc-800 dark:text-zinc-200">
          Contact Details
        </h2>
        <div className="flex items-center gap-2">
          {lead && (
            <div className="relative">
              <select
                value={lead.assignedTo || ''}
                onChange={(e) => updateLeadField({ assignedTo: e.target.value })}
                className="appearance-none pl-2.5 pr-6 py-1 rounded border border-zinc-200 dark:border-zinc-800 text-[10px] font-bold text-zinc-600 dark:text-zinc-400 bg-white/5 hover:bg-white/10 transition-colors cursor-pointer"
                title="Select Owner"
              >
                <option value="">Unassigned</option>
                {users.map((u) => (
                  <option key={u.id} value={u.email}>
                    {u.email.split('@')[0]}
                  </option>
                ))}
                {lead.assignedTo && !users.some(u => u.email === lead.assignedTo) && (
                  <option value={lead.assignedTo}>{lead.assignedTo.split('@')[0]}</option>
                )}
              </select>
              <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[6px] text-zinc-400 pointer-events-none">▼</span>
            </div>
          )}
          <button
            onClick={onClose}
            className="p-1 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 rounded-lg hover:bg-white/10 transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto no-scrollbar px-5 py-5">
        {!leadId ? (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <div className="grid h-10 w-10 place-items-center rounded-xl bg-white/30 dark:bg-white/5">
              <User className="h-4 w-4 text-zinc-400" />
            </div>
            <p className="text-[11px] font-semibold text-zinc-400">
              No lead linked to this conversation.
            </p>
          </div>
        ) : loading ? (
          <div className="grid h-32 place-items-center">
            <Loader2 className="h-5 w-5 animate-spin text-zinc-400" />
          </div>
        ) : lead ? (
          <div className="space-y-6">
            {/* Top Identity Block with Circle Avatar */}
            <div className="flex items-center gap-4">
              <span
                className="grid h-12 w-12 shrink-0 place-items-center rounded-full bg-gradient-to-br from-brand-500 to-violet-500 text-sm font-black text-white shadow-md"
                aria-hidden
              >
                {brandInitials(lead.name)}
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-2">
                  <h3 className="truncate text-base font-black text-zinc-900 dark:text-zinc-100">
                    {lead.name}
                  </h3>
                  {/* Owner selection control next to name */}
                  <div className="relative shrink-0">
                    <select
                      value={lead.assignedTo || ''}
                      onChange={(e) => updateLeadField({ assignedTo: e.target.value })}
                      className="appearance-none pl-2.5 pr-6 py-0.5 rounded border border-zinc-200 dark:border-zinc-800 text-[10px] font-bold text-zinc-600 dark:text-zinc-400 bg-white/5 hover:bg-white/10 transition-colors cursor-pointer"
                      title="Lead Owner"
                    >
                      <option value="">No Owner</option>
                      {users.map((u) => (
                        <option key={u.id} value={u.email}>
                          {u.email.split('@')[0]}
                        </option>
                      ))}
                      {lead.assignedTo && !users.some(u => u.email === lead.assignedTo) && (
                        <option value={lead.assignedTo}>{lead.assignedTo.split('@')[0]}</option>
                      )}
                    </select>
                    <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[6px] text-zinc-400 pointer-events-none">▼</span>
                  </div>
                </div>
                {/* Editable Status dropdown */}
                <div className="mt-1 flex items-center">
                  <div className="relative">
                    <select
                      value={lead.status}
                      onChange={(e) => updateLeadField({ status: e.target.value as any })}
                      className={cn(
                        "appearance-none pl-2 pr-5 py-0.5 rounded text-[10px] font-black uppercase tracking-wider border-0 cursor-pointer",
                        getLeadStatusStyles(lead.status)
                      )}
                    >
                      <option value="new">New</option>
                      <option value="contacted">Connected</option>
                      <option value="trial_booked">Trial</option>
                      <option value="member">Member</option>
                      <option value="dropped">Dropped</option>
                      <option value="paused">Paused</option>
                    </select>
                    <span className="absolute right-1.5 top-1/2 -translate-y-1/2 text-[7px] pointer-events-none opacity-60">▼</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Owner & Follower Selector matching Grow CRM screenshot */}
            <div className="grid grid-cols-2 gap-4 border-t border-b border-zinc-100 py-3 dark:border-white/5">
              <div>
                <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block mb-1">Owner</span>
                <div className="relative">
                  <select
                    value={lead.assignedTo || ''}
                    onChange={(e) => updateLeadField({ assignedTo: e.target.value })}
                    className="flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-zinc-200 dark:border-zinc-700 text-xs font-semibold text-zinc-700 dark:text-zinc-300 bg-white/5 hover:bg-white/10 transition-colors w-full justify-between appearance-none pr-8 cursor-pointer"
                  >
                    <option value="">Unassigned</option>
                    {users.map((u) => (
                      <option key={u.id} value={u.email}>
                        {u.email}
                      </option>
                    ))}
                    {lead.assignedTo && !users.some(u => u.email === lead.assignedTo) && (
                      <option value={lead.assignedTo}>{lead.assignedTo}</option>
                    )}
                  </select>
                  <span className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[8px] text-zinc-400 pointer-events-none">▼</span>
                </div>
              </div>
              <div>
                <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block mb-1">Followers</span>
                <div className="relative">
                  <select
                    value={follower}
                    onChange={(e) => {
                      setFollower(e.target.value);
                      localStorage.setItem(`projectx_lead_follower_${lead.id}`, e.target.value);
                    }}
                    className="flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-zinc-200 dark:border-zinc-700 text-xs font-semibold text-zinc-700 dark:text-zinc-300 bg-white/5 hover:bg-white/10 transition-colors w-full justify-between appearance-none pr-8 cursor-pointer"
                  >
                    <option value="">None</option>
                    {users.map((u) => (
                      <option key={u.id} value={u.email}>
                        {u.email}
                      </option>
                    ))}
                    {follower && !users.some(u => u.email === follower) && (
                      <option value={follower}>{follower}</option>
                    )}
                  </select>
                  <span className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[8px] text-zinc-400 pointer-events-none">▼</span>
                </div>
              </div>
            </div>

            {/* Tags section matching Grow CRM style */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[10px] font-black uppercase tracking-[0.1em] text-zinc-400">
                  Tags ({totalTagsCount})
                </span>
                {!isAddingTag && (
                  <button
                    onClick={() => setIsAddingTag(true)}
                    className="flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-black uppercase tracking-wider text-brand-500 hover:bg-brand-500/10 transition-colors"
                  >
                    <Plus className="h-2.5 w-2.5" />
                    Add
                  </button>
                )}
              </div>

              {isAddingTag && (
                <form onSubmit={handleAddTag} className="flex gap-1 animate-in fade-in duration-200">
                  <input
                    type="text"
                    value={newTagInput}
                    onChange={(e) => setNewTagInput(e.target.value)}
                    placeholder="New tag..."
                    className="flex-1 rounded border border-white/20 bg-white/35 px-2 py-1 text-[10px] font-semibold text-zinc-800 placeholder-zinc-400 focus:border-brand-500/40 focus:outline-none dark:border-white/5 dark:bg-white/5 dark:text-zinc-100"
                    autoFocus
                  />
                  <button
                    type="submit"
                    className="rounded bg-brand-500 px-2 py-1 text-[10px] font-bold text-white hover:bg-brand-600 transition-colors"
                  >
                    Add
                  </button>
                  <button
                    type="button"
                    onClick={() => setIsAddingTag(false)}
                    className="rounded bg-zinc-200 px-1.5 py-1 hover:bg-zinc-300 dark:bg-zinc-800 dark:hover:bg-zinc-700"
                  >
                    <X className="h-3.5 w-3.5 text-zinc-500" />
                  </button>
                </form>
              )}

              <div className="flex flex-wrap gap-1.5">
                {/* System Tags */}
                {systemTags.map((t, idx) => (
                  <span
                    key={`sys-${idx}`}
                    className={cn(
                      "px-2 py-0.5 rounded text-[9px] font-bold tracking-wide uppercase shrink-0 shadow-sm border border-black/5 dark:border-white/5",
                      t.style
                    )}
                  >
                    {t.label}
                  </span>
                ))}
                {/* Custom User-Added Tags */}
                {customTags.map((tag) => (
                  <span
                    key={`custom-${tag}`}
                    className="flex items-center gap-1 px-2 py-0.5 rounded text-[9px] font-bold tracking-wide uppercase bg-sky-500/15 text-sky-600 dark:bg-sky-500/25 dark:text-sky-400 shadow-sm border border-black/5 dark:border-white/5"
                  >
                    {tag}
                    <button
                      onClick={() => handleRemoveTag(tag)}
                      className="hover:text-rose-500 transition-colors"
                      title="Remove tag"
                    >
                      <X className="h-2 w-2" />
                    </button>
                  </span>
                ))}
                {totalTagsCount === 0 && (
                  <span className="text-[10px] font-medium text-zinc-400 italic">No tags</span>
                )}
              </div>
            </div>

            {/* Horizontal sub-tabs for All Fields, DND, Actions (Segmented Control style) */}
            <div className="bg-zinc-100 dark:bg-neutral-800/80 p-0.5 rounded-lg flex items-center justify-between gap-0.5">
              <button
                onClick={() => setActiveSubTab('fields')}
                className={cn(
                  "flex-1 py-1 rounded-md text-[10px] font-black uppercase tracking-wider text-center transition-all",
                  activeSubTab === 'fields'
                    ? "bg-white dark:bg-zinc-700 text-zinc-900 dark:text-white shadow-sm"
                    : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                )}
              >
                All fields
              </button>
              <button
                onClick={() => setActiveSubTab('dnd')}
                className={cn(
                  "flex-1 py-1 rounded-md text-[10px] font-black uppercase tracking-wider text-center transition-all",
                  activeSubTab === 'dnd'
                    ? "bg-white dark:bg-zinc-700 text-zinc-900 dark:text-white shadow-sm"
                    : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                )}
              >
                DND
              </button>
              <button
                onClick={() => setActiveSubTab('actions')}
                className={cn(
                  "flex-1 py-1 rounded-md text-[10px] font-black uppercase tracking-wider text-center transition-all",
                  activeSubTab === 'actions'
                    ? "bg-white dark:bg-zinc-700 text-zinc-900 dark:text-white shadow-sm"
                    : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                )}
              >
                Actions
              </button>
            </div>

            {/* Sub-tab content */}
            <div className="space-y-4 min-h-[120px]">
              {activeSubTab === 'fields' && (
                <div className="space-y-3 animate-in fade-in duration-200">
                  {/* Search box */}
                  <div className="relative">
                    <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-400">
                      <Search className="h-3.5 w-3.5" />
                    </span>
                    <input
                      type="text"
                      placeholder="Search fields and folders"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      className="w-full pl-8 pr-3 py-1.5 rounded-lg border border-zinc-200 dark:border-zinc-700 bg-white/30 dark:bg-white/5 text-[11px] font-medium text-zinc-800 placeholder-zinc-400 dark:text-zinc-200 focus:outline-none focus:ring-1 focus:ring-brand-500"
                    />
                  </div>

                  {/* Accordion list */}
                  <div className="space-y-2">
                    {/* Contact Section */}
                    <div className="border border-zinc-100 dark:border-white/5 rounded-xl overflow-hidden">
                      <button
                        type="button"
                        onClick={() => setExpandedSection(expandedSection === 'contact' ? null : 'contact')}
                        className="w-full flex items-center justify-between py-2 px-3 bg-zinc-50 dark:bg-zinc-800/50 text-[10px] font-black uppercase tracking-wider text-zinc-700 dark:text-zinc-200"
                      >
                        <span>Contact</span>
                        <span className="text-[9px] text-zinc-400">{expandedSection === 'contact' ? '▲' : '▼'}</span>
                      </button>
                      {expandedSection === 'contact' && (
                        <div className="p-3 bg-white/5 dark:bg-neutral-900/10 space-y-1">
                          <RowField label="First name" value={lead.firstName || (lead.name ? lead.name.split(' ')[0] : '')} />
                          <RowField label="Last name" value={lead.lastName || (lead.name && lead.name.split(' ').length > 1 ? lead.name.split(' ').slice(1).join(' ') : '')} />
                          <RowField label="Email" value={isPlaceholderEmail(lead.email) ? '' : lead.email} />
                          <RowField label="Phone" value={lead.phone} />
                          <RowField label="Contact source" value={lead.source} />
                          <RowField label="Referred by" value={lead.referrer} placeholder="--" />
                        </div>
                      )}
                    </div>

                    {/* Social Fitness Offer Details */}
                    <div className="border border-zinc-100 dark:border-white/5 rounded-xl overflow-hidden">
                      <button
                        type="button"
                        onClick={() => setExpandedSection(expandedSection === 'social' ? null : 'social')}
                        className="w-full flex items-center justify-between py-2 px-3 bg-zinc-50 dark:bg-zinc-800/50 text-[10px] font-black uppercase tracking-wider text-zinc-700 dark:text-zinc-200"
                      >
                        <span>Social Fitness Offer Details</span>
                        <span className="text-[9px] text-zinc-400">{expandedSection === 'social' ? '▲' : '▼'}</span>
                      </button>
                      {expandedSection === 'social' && (
                        <div className="p-3 bg-white/5 dark:bg-neutral-900/10 space-y-1">
                          <RowField label="Offer Claimed" value={lead.offer} placeholder="No active offers found." />
                          <RowField label="Fitness Goals" value={lead.goals} />
                          <RowField label="Notes" value={lead.notes} />
                          <RowField label="Further Notes" value={lead.furtherNotes} />
                        </div>
                      )}
                    </div>

                    {/* Hapana Fields */}
                    <div className="border border-zinc-100 dark:border-white/5 rounded-xl overflow-hidden">
                      <button
                        type="button"
                        onClick={() => setExpandedSection(expandedSection === 'hapana' ? null : 'hapana')}
                        className="w-full flex items-center justify-between py-2 px-3 bg-zinc-50 dark:bg-zinc-800/50 text-[10px] font-black uppercase tracking-wider text-zinc-700 dark:text-zinc-200"
                      >
                        <span>Hapana Fields</span>
                        <span className="text-[9px] text-zinc-400">{expandedSection === 'hapana' ? '▲' : '▼'}</span>
                      </button>
                      {expandedSection === 'hapana' && (
                        <div className="p-3 bg-white/5 dark:bg-neutral-900/10 space-y-1">
                          <RowField label="Fitness Plan" value={lead.fitnessPlan} placeholder="--" />
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}

              {activeSubTab === 'dnd' && (
                <div className="flex items-center gap-3 rounded-2xl border border-white/20 bg-white/20 p-3 dark:border-white/5 dark:bg-white/5 animate-in fade-in duration-200">
                  <button
                    onClick={toggleDND}
                    disabled={dndSaving}
                    className={cn(
                      'relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
                      lead.dndEnabled ? 'bg-rose-500' : 'bg-zinc-300 dark:bg-zinc-700',
                    )}
                    role="switch"
                    aria-checked={lead.dndEnabled}
                    title={lead.dndEnabled ? 'Turn off Do Not Disturb' : 'Turn on Do Not Disturb'}
                  >
                    <span
                      className={cn(
                        'pointer-events-none flex h-5 w-5 items-center justify-center rounded-full bg-white shadow-md ring-0 transition-transform duration-300 ease-out',
                        lead.dndEnabled ? 'translate-x-5' : 'translate-x-0.5',
                      )}
                    >
                      {dndSaving ? (
                        <Loader2 className="h-3 w-3 animate-spin text-zinc-500" />
                      ) : (
                        <BellOff className={cn('h-2.5 w-2.5', lead.dndEnabled ? 'text-rose-600' : 'text-zinc-400')} />
                      )}
                    </span>
                  </button>
                  <div className="min-w-0">
                    <div className="text-xs font-black uppercase tracking-wider text-zinc-700 dark:text-zinc-200">
                      Do Not Disturb
                    </div>
                    <p className="text-[10px] font-semibold leading-snug text-zinc-400">
                      {lead.dndEnabled
                        ? 'Automated messages are silenced.'
                        : 'Stops automated follow-ups and AI replies.'}
                    </p>
                  </div>
                </div>
              )}

              {activeSubTab === 'actions' && (
                <div className="space-y-2 animate-in fade-in duration-200">
                  <button className="w-full rounded-xl bg-zinc-100 hover:bg-zinc-200 dark:bg-white/5 dark:hover:bg-white/10 px-4 py-2.5 text-left text-xs font-bold text-zinc-700 dark:text-zinc-200 flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Mark Closed
                  </button>
                  <button className="w-full rounded-xl bg-zinc-100 hover:bg-zinc-200 dark:bg-white/5 dark:hover:bg-white/10 px-4 py-2.5 text-left text-xs font-bold text-zinc-700 dark:text-zinc-200 flex items-center gap-2">
                    <User className="h-3.5 w-3.5 text-blue-500" />
                    Reassign Owner
                  </button>
                </div>
              )}
            </div>

            {error && (
              <p className="text-[11px] font-bold text-rose-500 dark:text-rose-400">{error}</p>
            )}
          </div>
        ) : (
          <p className="py-10 text-center text-xs font-semibold text-zinc-400">
            Select a contact to view details.
          </p>
        )}
      </div>
    </aside>
  );
}

function RowField({ label, value, placeholder }: { label: string; value?: string; placeholder?: string }) {
  return (
    <div className="py-1.5 border-b border-zinc-100 dark:border-white/5 last:border-none">
      <span className="text-[9px] font-extrabold uppercase tracking-wider text-zinc-400 block">
        {label}
      </span>
      <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200 block mt-0.5">
        {value || placeholder || '--'}
      </span>
    </div>
  );
}

function isPlaceholderEmail(email?: string): boolean {
  if (!email) return true;
  // Auto-generated leads (WhatsApp/SMS/Messenger/Instagram inbound) get a
  // fabricated "<channel>-<contact>@example.com" address — see
  // emailPlaceholder in apps/api/internal/messaging/service.go.
  return email.endsWith('@example.com') || email.endsWith('@placeholder.com') || email.includes('placeholder');
}
