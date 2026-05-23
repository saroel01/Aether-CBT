import { writable } from 'svelte/store';

export interface ToastMessage {
  id: string;
  type: 'success' | 'warning' | 'error' | 'info';
  message: string;
}

const { subscribe, update } = writable<ToastMessage[]>([]);

export const toast = {
  subscribe,
  
  add: (message: string, type: ToastMessage['type'] = 'info', duration = 4000) => {
    const id = Math.random().toString(36).substring(2, 9);
    update((all) => [...all, { id, type, message }]);

    if (duration > 0) {
      setTimeout(() => {
        update((all) => all.filter((t) => t.id !== id));
      }, duration);
    }
  },
  
  remove: (id: string) => {
    update((all) => all.filter((t) => t.id !== id));
  },
  
  success: (msg: string, dur?: number) => toast.add(msg, 'success', dur),
  error: (msg: string, dur?: number) => toast.add(msg, 'error', dur),
  warning: (msg: string, dur?: number) => toast.add(msg, 'warning', dur),
  info: (msg: string, dur?: number) => toast.add(msg, 'info', dur)
};
