'use client';

import { useState } from 'react';
import { Plus, Users as UsersIcon, UserX, Loader2, Power } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Dialog, DialogHeader } from '@/components/ui/Dialog';
import { EmptyState } from '@/components/ui/EmptyState';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Label, FieldError } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { cn } from '@/lib/cn';
import { formatDate } from '@/lib/datetime';
import type { StudioRole, StudioUser } from '@/lib/types';
import { createStudioUser, deactivateStudioUser, setUserActive } from './actions';

const emptyForm = { username: '', firstName: '', lastName: '', email: '', roleId: '' };

// Same look/behavior as admin/studios/StudioStatusToggle and the Roles
// tab's RoleStatusToggle — a reversible switch, distinct from the
// permanent "Deactivate" button below. An inactive user simply can't log
// in (checked at login time) until switched back.
function UserStatusToggle({
  studioId,
  userId,
  userLabel,
  initialActive,
}: {
  studioId: string;
  userId: string;
  userLabel: string;
  initialActive: boolean;
}) {
  const [active, setActive] = useState(initialActive);
  const [loading, setLoading] = useState(false);

  async function handleToggle() {
    if (loading) return;
    setLoading(true);
    const next = !active;
    const res = await setUserActive(studioId, userId, next);
    setLoading(false);
    if (!res.ok) {
      alert(res.error);
      return;
    }
    setActive(next);
  }

  return (
    <button
      type="button"
      onClick={handleToggle}
      disabled={loading}
      className={cn(
        'relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        active ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700',
      )}
      role="switch"
      aria-checked={active}
      title={active ? `Deactivate ${userLabel}` : `Activate ${userLabel}`}
    >
      <span
        className={cn(
          'pointer-events-none flex h-5 w-5 items-center justify-center rounded-full bg-white shadow-md ring-0 transition-transform duration-300 ease-out',
          active ? 'translate-x-5' : 'translate-x-0.5',
        )}
      >
        {loading ? (
          <Loader2 className="h-3 w-3 animate-spin text-zinc-500" />
        ) : (
          <Power className={cn('h-2.5 w-2.5', active ? 'text-emerald-600' : 'text-zinc-400')} />
        )}
      </span>
    </button>
  );
}

export function UsersClient({
  studioId,
  users,
  roles,
}: {
  studioId: string;
  users: StudioUser[];
  roles: StudioRole[];
}) {
  const [creating, setCreating] = useState(false);
  const [deactivatingId, setDeactivatingId] = useState<string | null>(null);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-bold text-zinc-700 dark:text-zinc-300">
          {users.length} teammate{users.length === 1 ? '' : 's'}
        </h2>
        <Button
          size="sm"
          onClick={() => setCreating(true)}
          leftIcon={<Plus className="h-4 w-4" />}
          disabled={roles.length === 0}
          title={roles.length === 0 ? 'Create a role first' : undefined}
        >
          Add teammate
        </Button>
      </div>

      {roles.length === 0 && (
        <p className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300">
          Create a role on the Roles tab before adding teammates.
        </p>
      )}

      <Dialog open={creating} onClose={() => setCreating(false)} widthClassName="max-w-xl">
        <DialogHeader icon={<UsersIcon className="h-5 w-5 text-brand-500" />} title="New teammate" onClose={() => setCreating(false)} />
        <CreateUserForm studioId={studioId} roles={roles} onDone={() => setCreating(false)} />
      </Dialog>

      {users.length === 0 && !creating && (
        <EmptyState
          icon={<UsersIcon className="h-6 w-6" />}
          title="No teammates yet"
          description="Add a teammate to give them their own login, scoped to what their role allows."
        />
      )}

      {users.length > 0 && (
        <Card noPadding>
          <div className="overflow-x-auto">
            <table className="min-w-[720px] w-full text-sm">
              <thead className="border-b border-zinc-200 text-left text-xs font-bold uppercase tracking-wide text-zinc-400 dark:border-zinc-800">
                <tr>
                  <th className="px-4 py-3">Name</th>
                  <th className="px-4 py-3">Email</th>
                  <th className="px-4 py-3">Role</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3">Added</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100 dark:divide-zinc-800">
                {users.map((u) => (
                  <tr key={u.id}>
                    <td className="px-4 py-3">
                      <div className="font-semibold text-zinc-900 dark:text-zinc-100">
                        {u.firstName || u.lastName ? `${u.firstName ?? ''} ${u.lastName ?? ''}`.trim() : '—'}
                      </div>
                      {u.username && <div className="text-xs text-zinc-500 dark:text-zinc-400">@{u.username}</div>}
                    </td>
                    <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">{u.email}</td>
                    <td className="px-4 py-3">
                      {u.role === 'studio_admin' ? (
                        <Badge tone="brand">Studio Admin</Badge>
                      ) : (
                        <Badge tone="neutral">{u.roleName ?? '—'}</Badge>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {!u.active ? (
                        <Badge tone="neutral">Inactive</Badge>
                      ) : u.mustResetPassword ? (
                        <Badge tone="warning">Awaiting password reset</Badge>
                      ) : (
                        <Badge tone="success">Active</Badge>
                      )}
                    </td>
                    <td className="px-4 py-3 text-zinc-500 dark:text-zinc-400">{formatDate(u.createdAt)}</td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-3">
                        <UserStatusToggle
                          studioId={studioId}
                          userId={u.id}
                          userLabel={u.email}
                          initialActive={u.active}
                        />
                        {u.role !== 'studio_admin' && (
                          <Button
                            size="sm"
                            variant="danger"
                            disabled={deactivatingId === u.id}
                            leftIcon={<UserX className="h-3.5 w-3.5" />}
                            onClick={async () => {
                              if (!confirm(`Delete ${u.email}? This can't be undone.`)) return;
                              setDeactivatingId(u.id);
                              const res = await deactivateStudioUser(studioId, u.id);
                              setDeactivatingId(null);
                              if (!res.ok) alert(res.error);
                            }}
                          >
                            Delete
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  );
}

function CreateUserForm({
  studioId,
  roles,
  onDone,
}: {
  studioId: string;
  roles: StudioRole[];
  onDone: () => void;
}) {
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    const errs: Record<string, string> = {};
    if (!form.username.trim()) errs.username = 'required';
    if (!form.firstName.trim()) errs.firstName = 'required';
    if (!form.lastName.trim()) errs.lastName = 'required';
    if (!form.email.trim()) errs.email = 'required';
    if (!form.roleId) errs.roleId = 'required';
    if (Object.keys(errs).length > 0) {
      setErrors(errs);
      return;
    }
    setSaving(true);
    try {
      const res = await createStudioUser(studioId, form);
      if (!res.ok) {
        setErrors(res.details ?? { _: res.error });
        return;
      }
      onDone();
    } finally {
      setSaving(false);
    }
  }

  const selectedRole = roles.find((r) => r.id === form.roleId) ?? null;

  return (
      <form onSubmit={onSubmit} className="max-h-[calc(92vh-64px)] space-y-4 overflow-y-auto px-5 py-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <Label>First name</Label>
            <Input value={form.firstName} onChange={(e) => setForm((f) => ({ ...f, firstName: e.target.value }))} invalid={!!errors.firstName} />
            <FieldError message={errors.firstName} />
          </div>
          <div>
            <Label>Last name</Label>
            <Input value={form.lastName} onChange={(e) => setForm((f) => ({ ...f, lastName: e.target.value }))} invalid={!!errors.lastName} />
            <FieldError message={errors.lastName} />
          </div>
          <div>
            <Label>Username</Label>
            <Input value={form.username} onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))} placeholder="e.g. jsmith" invalid={!!errors.username} />
            <FieldError message={errors.username} />
          </div>
          <div>
            <Label>Email</Label>
            <Input type="email" value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} invalid={!!errors.email} />
            <FieldError message={errors.email} />
          </div>
          <div>
            <Label>Role</Label>
            <Select value={form.roleId} onChange={(e) => setForm((f) => ({ ...f, roleId: e.target.value }))} invalid={!!errors.roleId}>
              <option value="">Select a role…</option>
              {roles.map((r) => (
                <option key={r.id} value={r.id}>{r.name}{r.active ? '' : ' (inactive)'}</option>
              ))}
            </Select>
            <FieldError message={errors.roleId} />
          </div>
        </div>

        {selectedRole && (
          <div>
            <Label>Access this role grants</Label>
            {!selectedRole.active && (
              <p className="mb-1.5 text-xs text-amber-600 dark:text-amber-400">
                This role is currently inactive — the teammate won&rsquo;t be able to log in until it&rsquo;s reactivated.
              </p>
            )}
            <div className="flex flex-wrap gap-1.5">
              {selectedRole.permissionKeys.length === 0 ? (
                <Badge tone="neutral">No access granted</Badge>
              ) : (
                selectedRole.permissionKeys.map((key) => (
                  <Badge key={key} tone="brand">
                    {key.split('-').map((w) => (w ? w[0]!.toUpperCase() + w.slice(1) : w)).join(' ')}
                  </Badge>
                ))
              )}
            </div>
          </div>
        )}

        <p className="text-xs text-zinc-500 dark:text-zinc-400">
          They&rsquo;ll be able to log in with this email and a default password, and will be asked to set
          their own password immediately.
        </p>

        {errors._ && <FieldError message={errors._} />}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onDone}>Cancel</Button>
          <Button type="submit" disabled={saving} suppressHydrationWarning>
            {saving ? 'Adding…' : 'Add teammate'}
          </Button>
        </div>
      </form>
  );
}
