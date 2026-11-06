import { NextRequest, NextResponse } from 'next/server';

// CSP violation-report endpoint (platform-security-hardening task 8.2)
// — the browser POSTs here automatically whenever a page violates the
// Content-Security-Policy set in src/middleware.ts's report-uri
// directive. Logged only, never authenticated/authorized — a browser
// sends this unauthenticated by design, same as any other `report-uri`
// receiver.
export async function POST(request: NextRequest) {
  let body: unknown = null;
  try {
    body = await request.json();
  } catch {
    // Some browsers send `application/csp-report` with a body that
    // isn't strict JSON in older report formats — don't fail the
    // request over a parse quirk, just log that it happened.
    console.warn('[csp-report] received a non-JSON violation report body');
  }
  console.warn('[csp-report] CSP violation reported', JSON.stringify(body));
  // 204 — browsers don't do anything with the response body for
  // report-uri submissions.
  return new NextResponse(null, { status: 204 });
}
