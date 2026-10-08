// Deterministic date/time formatting for SSR.
//
// Why this file exists: `new Date(iso).toLocaleDateString()` (and friends)
// without an explicit locale uses the *runtime's* default locale — which
// differs between the Next.js server (typically en-GB / en-US depending on
// the host OS) and the user's browser. That difference makes the server
// HTML and the client's first render disagree → React hydration mismatch.
//
// Every Date → string in the UI goes through one of these helpers. They all
// pin a fixed locale so server and client always render the same string.

const LOCALE = 'en-GB'; // unambiguous "DD MMM YYYY" / 24h time. Stable choice.

/** "7 May 2026" — short, unambiguous date. Use for "created at", "connected" etc. */
export function formatDate(iso: string | Date): string {
  return toDate(iso).toLocaleDateString(LOCALE, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

/** "7 May 2026, 14:32" — date + 24h time. Use for full timestamps. */
export function formatDateTime(iso: string | Date): string {
  return toDate(iso).toLocaleString(LOCALE, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/** "14:32" — 24h time only. */
export function formatTime(iso: string | Date): string {
  return toDate(iso).toLocaleTimeString(LOCALE, {
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Relative time ("2m ago", "3h ago", "Yesterday"). Inherently changes between
 * server render and client render because `Date.now()` advances. Callers MUST
 * either:
 *   1. wrap the rendered span with `suppressHydrationWarning`, or
 *   2. only call this from a `useEffect` after mount.
 *
 * For tightly-formatted output (chat timestamps, list rows) we go with #1 —
 * the visual difference is at most one bucket boundary and React quietly
 * accepts it.
 */
export function relativeTime(iso: string | Date): string {
  const diff = Date.now() - toDate(iso).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return 'now';
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}d`;
  return formatDate(iso);
}

function toDate(v: string | Date): Date {
  return v instanceof Date ? v : new Date(v);
}

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

/** Calendar days between `iso` and now (0 = today, 1 = yesterday, ...). Can be negative for a future date. */
function daysAgo(iso: string | Date): number {
  const d = toDate(iso);
  return Math.round((startOfDay(new Date()) - startOfDay(d)) / 86400000);
}

/** WhatsApp Web–style conversation-list timestamp: time for today, "Yesterday", weekday
 * name for the rest of the past week, short date beyond that. */
export function formatChatListTimestamp(iso: string | Date): string {
  const d = daysAgo(iso);
  if (d <= 0) return formatTime(iso);
  if (d === 1) return 'Yesterday';
  if (d < 7) return toDate(iso).toLocaleDateString(LOCALE, { weekday: 'long' });
  return formatDate(iso);
}

/** WhatsApp Web–style day-divider label: "Today" / "Yesterday" / weekday / short date. */
export function formatDayDivider(iso: string | Date): string {
  const d = daysAgo(iso);
  if (d <= 0) return 'Today';
  if (d === 1) return 'Yesterday';
  if (d < 7) return toDate(iso).toLocaleDateString(LOCALE, { weekday: 'long' });
  return formatDate(iso);
}

/** Calendar-day key (e.g. "2026-10-07") for grouping messages by day. */
export function dayKey(iso: string | Date): string {
  const d = toDate(iso);
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}
