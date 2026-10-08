// Turns the raw error stored on a failed outbound job (or broadcast recipient) into a
// short, human reason. Raw errors from wa-web arrive as
// "wa-web send failed <code>: {...json...}", from the network layer as
// `Post "http://...": dial tcp: lookup ...`, or as short codes like daily_limit_exceeded.
export function jobFailureReason(lastError?: string | null): string {
  if (!lastError) return 'Send failed';
  const raw = lastError.trim();
  if (raw === 'daily_limit_exceeded') return 'WhatsApp daily message limit reached';

  const jsonStart = raw.indexOf('{');
  if (jsonStart !== -1) {
    try {
      const parsed = JSON.parse(raw.slice(jsonStart));
      const inner: string = parsed?.error || parsed?.message || '';
      if (inner) {
        if (/not registered on whatsapp/i.test(inner)) return 'Not on WhatsApp';
        if (/session not connected/i.test(inner)) return 'WhatsApp is disconnected';
        if (/rate limit/i.test(inner)) return 'WhatsApp rate limit hit';
        return inner;
      }
    } catch {
      // Not valid JSON after all — fall through to the checks below.
    }
  }
  if (/not registered on whatsapp/i.test(raw)) return 'Not on WhatsApp';
  if (/dnd enabled/i.test(raw)) return 'Do Not Disturb is on';
  if (/no active channel/i.test(raw)) return 'Channel is not connected';
  if (/credentials/i.test(raw)) return 'Channel credentials are invalid';
  if (/dial tcp|lookup |server misbehaving|no such host|connection (refused|reset)|i\/o timeout|deadline exceeded/i.test(raw)) {
    return 'Network problem, will retry';
  }
  return raw;
}
