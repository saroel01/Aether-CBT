// src/lib/api.ts
// Centralized API client for Aether CBT backend. `api()` delegates to the auth-aware
// `apiFetch` wrapper (api-auth.ts) which handles 401 redirect + timeout centrally
// (Task 19). Low-level helpers (apiUrl, authHeaders, qrCodeUrl) stay here so callers
// that build their own fetch (file uploads, EventSource) can reuse them.

import { apiFetch } from './api-auth';

const configuredApiBase = import.meta.env.VITE_API_BASE as string | undefined;
// Relative '/api' keeps every request same-origin with the page: in dev, Vite's server.proxy
// (vite.config.js) forwards /api/* to the Go backend, which fixes cross-origin frame blocking
// for the exam iframe and the cross-port SameSite cookie. In production the backend serves the
// built SPA itself, so '/api' is naturally same-origin too. Set VITE_API_BASE to an absolute URL
// to bypass the proxy and target a different backend.
const API_BASE = configuredApiBase || '/api';

function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('aether_token');
}

// H4: only an explicitly configured tenant is sent. Without one the backend resolves
// the tenant from subdomain/slug, and the JWT claim always overrides after login.
function getTenantID(): string | null {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('aether_tenant_id') || import.meta.env.VITE_TENANT_ID || null;
  }
  return import.meta.env.VITE_TENANT_ID || null;
}

export function apiUrl(path: string): string {
  return `${API_BASE}${path}`;
}

export function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const token = getToken();
  const tenantID = getTenantID();
  const headers: Record<string, string> = {
    ...(tenantID ? { 'X-Tenant-ID': tenantID } : {}),
    ...extra
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

export function qrCodeUrl(text: string): string {
  return apiUrl(`/qrcode?text=${encodeURIComponent(text)}`);
}

/**
 * Builds a QR Code image URL pointing directly to the student login page
 * with pre-filled exam token and optional student ID.
 */
export function studentLoginQrUrl(token: string, noId?: string): string {
  const origin = typeof window !== 'undefined' && window.location?.origin ? window.location.origin : '';
  const params = new URLSearchParams();
  if (noId) params.set('no_id', noId);
  if (token) params.set('token', token);
  const path = `/student/login?${params.toString()}`;
  return qrCodeUrl(origin ? `${origin}${path}` : path);
}

// api delegates to the auth-aware fetch wrapper so every caller gets centralized 401
// handling + timeout. Callers that need the raw 401 (login) pass { raw401: true }.
export async function api<T = any>(path: string, options: RequestInit & { raw401?: boolean } = {}): Promise<T> {
  return apiFetch<T>(path, options);
}

export const auth = {
  login: (username: string, password: string) =>
    api<{ success: boolean; data: { token: string; user: any } }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
      raw401: true
    }),

  studentLogin: (no_id: string, password: string, token: string) =>
    api<{ success: boolean; data: { peserta_id: number; token: string; user?: any } }>('/auth/student-login', {
      method: 'POST',
      body: JSON.stringify({ no_id, password, token }),
      raw401: true
    }),

  logout: () => {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('aether_token');
      localStorage.removeItem('aether_user');
    }
  }
};
