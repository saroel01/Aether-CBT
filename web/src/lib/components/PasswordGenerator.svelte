<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Button from './ui/Button.svelte';

  export let value = '';
  export let length = 12;
  export let label = 'Password';
  export let placeholder = 'Masukkan password';
  export let theme: 'light' | 'dark' = 'light';
  export let id = 'password-generator-input';

  const dispatch = createEventDispatcher();

  function generateStrongPassword(len: number = 12): string {
    const lower = 'abcdefghijklmnopqrstuvwxyz';
    const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    const digits = '0123456789';
    const symbols = '!@#$%^&*()-_=+';

    const all = lower + upper + digits + symbols;

    // Pastikan minimal ada 1 dari setiap kategori
    let password = '';
    password += lower[Math.floor(Math.random() * lower.length)];
    password += upper[Math.floor(Math.random() * upper.length)];
    password += digits[Math.floor(Math.random() * digits.length)];
    password += symbols[Math.floor(Math.random() * symbols.length)];

    // Isi sisanya
    const array = new Uint8Array(len - 4);
    crypto.getRandomValues(array);

    for (let i = 0; i < array.length; i++) {
      password += all[array[i] % all.length];
    }

    // Shuffle
    return password
      .split('')
      .sort(() => Math.random() - 0.5)
      .join('');
  }

  function generate() {
    value = generateStrongPassword(length);
    dispatch('generate', { value });
  }

  function copyToClipboard() {
    if (!value) return;
    navigator.clipboard.writeText(value).then(() => {
      // Dispatch or success alert
    });
  }

  let themeClasses = {
    dark: 'text-slate-100 bg-slate-950/40 border-slate-800 hover:border-slate-700 focus:border-indigo-500 focus:ring-indigo-500/10 placeholder-slate-600',
    light: 'text-slate-900 bg-slate-50/50 border-slate-200/80 hover:border-slate-300 focus:border-indigo-600 focus:ring-indigo-600/10 placeholder-slate-400'
  };

  let labelThemeClasses = {
    dark: 'text-slate-400',
    light: 'text-slate-500'
  };
</script>

<div class="space-y-1.5 w-full">
  <label for={id} class="text-xs font-semibold uppercase tracking-widest {labelThemeClasses[theme]}">
    {label}
  </label>

  <div class="flex gap-2">
    <div class="relative flex-1">
      <input
        {id}
        type="password"
        bind:value
        {placeholder}
        class="w-full h-12 px-4 pr-10 border rounded-2xl outline-none transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] focus:ring-4 {themeClasses[theme]}"
      />
      {#if value}
        <button
          type="button"
          on:click={copyToClipboard}
          aria-label="Salin password"
          class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200 z-10 flex items-center transition-colors"
          title="Salin password"
        >
          📋
        </button>
      {/if}
    </div>

    <Button 
      variant="secondary" 
      size="md"
      {theme}
      on:click={generate}
      class="whitespace-nowrap rounded-2xl font-semibold"
    >
      Generate
    </Button>
  </div>

  <p class="text-[10px] text-slate-400 leading-normal">
    Klik Generate untuk membuat password kuat otomatis (huruf, angka, simbol).
  </p>
</div>
