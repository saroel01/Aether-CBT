<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Button from './ui/Button.svelte';

  export let value = '';
  export let length = 12;
  export let label = 'Password';
  export let placeholder = 'Masukkan password';

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
      // Bisa ditambahkan toast jika diperlukan
    });
  }
</script>

<div class="space-y-1.5">
  <label class="text-xs font-semibold text-slate-500 uppercase tracking-wider">
    {label}
  </label>

  <div class="flex gap-2">
    <div class="relative flex-1">
      <input
        type="password"
        bind:value
        {placeholder}
        class="w-full h-11 px-4 pr-10 text-slate-800 bg-white border border-slate-200/80 rounded-xl outline-none transition-all focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      />
      {#if value}
        <button
          type="button"
          on:click={copyToClipboard}
          class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
          title="Salin password"
        >
          📋
        </button>
      {/if}
    </div>

    <Button 
      variant="secondary" 
      size="md"
      on:click={generate}
      class="whitespace-nowrap"
    >
      Generate
    </Button>
  </div>

  <p class="text-[10px] text-slate-400">
    Klik Generate untuk membuat password kuat otomatis (huruf, angka, simbol).
  </p>
</div>
