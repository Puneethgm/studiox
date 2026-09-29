// Browser-side API helper.
//
// All requests use *relative* paths (e.g. `/api/v1/auth/login`). The browser
// resolves them against `window.location.origin` automatically, and Next.js's
// rewrite rule (see next.config.mjs) proxies `/api/*` to the Go backend in
// dev — same-origin in prod via the deploy's ingress. There is intentionally
// no hardcoded API host in client code.

export class ApiError extends Error {
  status: number;
  code?: string;
  details?: Record<string, string>;

  constructor(message: string, status: number, code?: string, details?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

interface ApiOptions extends Omit<RequestInit, 'body'> {
  json?: unknown;
}

export async function api<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const { json, headers, ...rest } = opts;
  const res = await fetch(path, {
    ...rest,
    credentials: 'include',
    headers: {
      ...(json ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: json ? JSON.stringify(json) : (rest as RequestInit).body,
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  const body = text ? JSON.parse(text) : undefined;

  if (res.status === 401 && body?.code === 'unauthorized' && typeof window !== 'undefined') {
    // Expired JWT, idle session timeout, or revoked session — a full
    // navigation to /login rather than a client-side router push, so any
    // stale in-memory state from the dead session doesn't linger. Checked
    // by code, not just status: a failed *login attempt* is also a 401 but
    // uses invalid_credentials/account_inactive/role_inactive — those must
    // stay on the login form as an inline error, not bounce through here.
    window.location.href = '/login';
  }

  if (!res.ok) {
    throw new ApiError(
      body?.error ?? `HTTP ${res.status}`,
      res.status,
      body?.code,
      body?.details,
    );
  }
  return body as T;
}
