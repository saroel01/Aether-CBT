// src/lib/api.ts
// Centralized API client for Aether CBT backend

const API_BASE = 'http://localhost:3000/api';

function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('aether_token');
}

export async function api<T = any>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {})
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  // Default tenant for development (can be overridden via X-Tenant-* headers)
  if (!headers['X-Tenant-ID'] && !headers['X-Tenant-Slug']) {
    headers['X-Tenant-ID'] = '1';
  }

  const res = await fetch(`${API_BASE}${path}`, {
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
    api<{ success: boolean; data: { peserta_id: number } }>('/auth/student-login', {
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
