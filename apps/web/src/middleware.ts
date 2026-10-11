import { NextRequest, NextResponse } from 'next/server';

// requireSession / serverFetch (lib/auth.ts) redirect to /login on a dead
// session, but a Server Component has no built-in way to read the path it's
// currently rendering. This stamps it onto a request header so those
// redirects can carry it as /login?next=<path>, letting the login page send
// the user back to the page they were on instead of always landing on the
// role default. Scoped to /admin/* — the only auth-gated tree.
export function middleware(request: NextRequest) {
  const headers = new Headers(request.headers);
  headers.set('x-pathname', request.nextUrl.pathname + request.nextUrl.search);
  return NextResponse.next({ request: { headers } });
}

export const config = {
  matcher: '/admin/:path*',
};
