'use client';

import { useEffect, useRef } from 'react';
import { api } from './api';

// Keeps the session alive (via RequireAuth's sliding Refresh — see
// apps/api/internal/identity/jwt.go) only while the user is actually
// present: real mouse/keyboard/scroll/touch activity, not just an open tab.
// Without this, a user reading a long page without clicking anything would
// generate zero API traffic and get logged out by the server's idle timer
// despite being "active" by any human definition.
//
// If nothing happens for IDLE_THRESHOLD_MS, heartbeats simply stop and the
// server's own SESSION_IDLE_TIMEOUT (default 30m — see config.go) takes it
// from there and logs the session out on its own; this hook never forces a
// logout itself. IDLE_THRESHOLD_MS is kept comfortably below that 30-minute
// server default so a heartbeat always lands well before the server would
// otherwise consider the session idle.
const HEARTBEAT_INTERVAL_MS = 2 * 60 * 1000; // 2 minutes
const IDLE_THRESHOLD_MS = 25 * 60 * 1000; // 25 minutes

const ACTIVITY_EVENTS = ['mousemove', 'keydown', 'scroll', 'click', 'touchstart'] as const;

export function useIdleHeartbeat(): void {
  const lastActivityRef = useRef(Date.now());

  useEffect(() => {
    const onActivity = () => {
      lastActivityRef.current = Date.now();
    };
    for (const evt of ACTIVITY_EVENTS) {
      window.addEventListener(evt, onActivity, { passive: true });
    }

    const interval = setInterval(() => {
      if (Date.now() - lastActivityRef.current < IDLE_THRESHOLD_MS) {
        // Any authenticated endpoint works — this one is already the
        // cheapest read in the API. A failure here (including a genuine
        // 401) is handled by api()'s own session-expiry logic; nothing
        // further to do in either case.
        api('/api/v1/auth/me').catch(() => {});
      }
    }, HEARTBEAT_INTERVAL_MS);

    return () => {
      for (const evt of ACTIVITY_EVENTS) {
        window.removeEventListener(evt, onActivity);
      }
      clearInterval(interval);
    };
  }, []);
}
