// src/lib/api-auth.ts
// Auth-aware fetch wrapper: centralizes request headers, a 15s timeout, and 401 handling.
// On a 401 (token expired/invalid) it clears the auth store and redirects to the
// role-appropriate login page, so pages stop retrying forever with a dead token
// (review frontend finding #4, Task 19). Login callers pass `raw401: true` so a wrong
// password surfaces as a normal error instead of triggering a redirect.

import { authHeaders, apiUrl } from './api';
import { authStore } from './stores/auth';

export interface ApiOptions extends RequestInit {
  /** If true, a 401 throws rather than clearing auth + redirecting (for the login call). */
  raw401?: boolean;
}

/** Default request timeout (ms). AbortController cancels the fetch if it exceeds this. */
const REQUEST_TIMEOUT_MS = 15000;

/**
 * Auth-aware fetch. Wraps window.fetch with:
 *   - JSON content type + auth headers (token, tenant),
 *   - a 15-second AbortController timeout,
 *   - centralized 401 handling (clear + redirect) unless `raw401` is set,
 *   - a typed JSON (or text) result.
 */
export async function apiFetch<T = any>(path: string, options: ApiOptions = {}): Promise<T> {
  const { raw401, ...fetchOpts } = options;
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((fetchOpts.headers as Record<string, string>) || {})
  };
  Object.assign(headers, authHeaders());

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
  try {
    const res = await fetch(apiUrl(path), { ...fetchOpts, headers, signal: controller.signal });

    if (res.status === 401 && !raw401) {
      // Token expired or invalid: clear and redirect to the role-appropriate login so the
      // user re-authenticates instead of seeing endless failed retries.
      authStore.logout();
      if (typeof window !== 'undefined') {
        const p = window.location.pathname.replace(/\/+$/, '').toLowerCase() || '/';
        const login = (p.startsWith('/student') ? '/student/login'
          : p.startsWith('/supervisor') ? '/supervisor/login'
          : '/admin').toLowerCase();
        if (p !== login) {
          window.location.href = login;
        }
      }
      throw new Error('Sesi berakhir, silakan login kembali');
    }

    if (!res.ok) {
      let msg = 'Request failed';
      try {
        const e = await res.json();
        msg = e.error || e.message || msg;
      } catch {
        /* non-JSON error body; keep the default message */
      }
      throw new Error(msg);
    }

    // Some endpoints (CSV/blob) return non-JSON; let the caller handle the body.
    const ct = res.headers.get('content-type') || '';
    if (ct.includes('application/json')) return res.json();
    return res.text() as unknown as T;
  } finally {
    clearTimeout(timeout);
  }
}
