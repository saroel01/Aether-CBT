// src/lib/stores/auth.ts
import { writable } from 'svelte/store';

interface User {
  id: number;
  username: string;
  role: string;
  full_name?: string;
  tenant_id: number;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    token: null,
    isAuthenticated: false
  });

  // Load from localStorage on init (browser only)
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('aether_token');
    const userStr = localStorage.getItem('aether_user');

    if (token && userStr) {
      try {
        const user = JSON.parse(userStr);
        set({ user, token, isAuthenticated: true });
      } catch {
        // corrupted storage, clear it
        localStorage.removeItem('aether_token');
        localStorage.removeItem('aether_user');
      }
    }
  }

  return {
    subscribe,

    login: (token: string, user: User) => {
      if (typeof window !== 'undefined') {
        localStorage.setItem('aether_token', token);
        localStorage.setItem('aether_user', JSON.stringify(user));
      }
      set({ user, token, isAuthenticated: true });
    },

    logout: () => {
      if (typeof window !== 'undefined') {
        localStorage.removeItem('aether_token');
        localStorage.removeItem('aether_user');
      }
      set({ user: null, token: null, isAuthenticated: false });
    }
  };
}

export const authStore = createAuthStore();
