// src/lib/api.ts
// Centralized API client for Aether CBT backend

const configuredApiBase = import.meta.env.VITE_API_BASE as string | undefined;
const API_BASE = configuredApiBase || (typeof window !== 'undefined' && window.location.port === '5173' ? 'http://localhost:3000/api' : '/api');

function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('aether_token');
}

function getTenantID(): string {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('aether_tenant_id') || import.meta.env.VITE_TENANT_ID || '1';
  }
  return import.meta.env.VITE_TENANT_ID || '1';
}

export function apiUrl(path: string): string {
  return `${API_BASE}${path}`;
}

export function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const token = getToken();
  const headers: Record<string, string> = {
    'X-Tenant-ID': getTenantID(),
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

export async function api<T = any>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {})
  };

  Object.assign(headers, authHeaders());

  const res = await fetch(apiUrl(path), {
    ...options,
    headers
  });

  if (!res.ok) {
    let errorMessage = 'Request failed';
    try {
      const err = await res.json();
      errorMessage = err.error || err.message || errorMessage;
    } catch {}
    throw new Error(errorMessage);
  }

  return res.json();
}

export const auth = {
  login: (username: string, password: string) =>
    api<{ success: boolean; data: { token: string; user: any } }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password })
    }),

  studentLogin: (no_id: string, password: string, token: string) =>
    api<{ success: boolean; data: { peserta_id: number; token: string; user?: any } }>('/auth/student-login', {
      method: 'POST',
      body: JSON.stringify({ no_id, password, token })
    }),

  logout: () => {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('aether_token');
      localStorage.removeItem('aether_user');
    }
  }
};

export const students = {
  list: () => api('/students'),
  // add create later
};

export const classes = {
  list: () => api('/classes'),
  create: (nama_kelas: string) =>
    api('/classes', { method: 'POST', body: JSON.stringify({ nama_kelas }) })
};

export const mapel = {
  list: () => api('/mapel'),
  create: (nama_mapel: string, kode_mapel?: string) =>
    api('/mapel', { method: 'POST', body: JSON.stringify({ nama_mapel, kode_mapel }) })
};

export const rooms = {
  list: () => api('/rooms'),
  create: (nama_ruang: string, username: string, password: string) =>
    api('/rooms', { method: 'POST', body: JSON.stringify({ nama_ruang, username, password }) })
};
