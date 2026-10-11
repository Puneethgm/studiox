// Shared helpers for surviving a forced /login redirect (idle session
// timeout, revoked session) without silently dropping whatever the user
// was in the middle of.
//
// Two separate concerns live here:
//  1. A `next` path so /login can send the user back to the page they were
//     on, instead of always landing on the role default.
//  2. A "session is about to die" event + tiny localStorage draft helpers,
//     so a page holding not-yet-saved client state (a form mid-edit, a
//     builder with pending changes) gets one last synchronous chance to
//     stash it before the full navigation to /login tears the page down.

const SESSION_EXPIRING_EVENT = 'studiox:session-expiring';
const DRAFT_PREFIX = 'studiox:draft:';

// Only ever redirect back into the admin app — never follow a `next` value
// pointing off-site (open-redirect) or back to /login itself.
export function sanitizeNextPath(raw: string | null | undefined): string | null {
  if (!raw) return null;
  if (!raw.startsWith('/admin/') && raw !== '/admin') return null;
  if (raw.startsWith('//')) return null;
  return raw;
}

export function notifySessionExpiring(): void {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event(SESSION_EXPIRING_EVENT));
}

// Subscribes cb to the session-expiring event. cb must be synchronous (and
// fast) — by the time it fires, the session is already dead and the page
// is about to unmount. Returns an unsubscribe function.
export function onSessionExpiring(cb: () => void): () => void {
  if (typeof window === 'undefined') return () => {};
  window.addEventListener(SESSION_EXPIRING_EVENT, cb);
  return () => window.removeEventListener(SESSION_EXPIRING_EVENT, cb);
}

export function saveDraft<T>(key: string, value: T): void {
  try {
    window.localStorage.setItem(DRAFT_PREFIX + key, JSON.stringify({ value, savedAt: Date.now() }));
  } catch {
    // localStorage can throw (private mode, quota, disabled) — losing the
    // rescue draft is strictly better than crashing the expiring page.
  }
}

export function loadDraft<T>(key: string): T | null {
  try {
    const raw = window.localStorage.getItem(DRAFT_PREFIX + key);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { value: T; savedAt: number };
    return parsed.value;
  } catch {
    return null;
  }
}

export function clearDraft(key: string): void {
  try {
    window.localStorage.removeItem(DRAFT_PREFIX + key);
  } catch {
    // ignore
  }
}

// For a reusable row/field component (EditableTextRow, ColorEditRow, ...)
// that has no natural unique id prop (studioId, treeId, ...) to key a
// draft on: combines the current path with a local label so the same
// component rendered on two different pages — or twice on one page with a
// different label — never collides.
export function pageScopedDraftKey(label: string): string {
  if (typeof window === 'undefined') return label;
  return `${window.location.pathname}:${label}`;
}

// A Server Action's 401 (see e.g. settings/actions.ts) never goes through
// lib/api.ts's client-side interception — it comes back as an ordinary
// `{ ok: false, code: 'unauthorized' }` result, not a thrown ApiError or a
// browser-level redirect. Call this from the spot that just got that result
// to get the same behavior client fetches get automatically: full
// navigation to /login, carrying `next` back to the current page.
export function redirectToLoginFromHere(): void {
  if (typeof window === 'undefined') return;
  const next = window.location.pathname + window.location.search;
  window.location.href = `/login?next=${encodeURIComponent(next)}`;
}
