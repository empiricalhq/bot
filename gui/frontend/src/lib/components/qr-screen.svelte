<script lang="ts">
  import HeadlessQr from '$lib/components/component.svelte';
  import { botStore } from '$lib/stores/bot.store';

  const { qrCode, status, error } = $derived($botStore);
</script>

<div
  class="flex h-screen items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-950"
>
  <div class="w-full max-w-md px-6">
    {#if status === 'connecting'}
      <div class="text-center">
        <div class="relative mx-auto mb-8 h-32 w-32">
          <div class="absolute inset-0 animate-ping rounded-full bg-primary/20"></div>
          <div
            class="relative flex h-full items-center justify-center rounded-full bg-white shadow-xl dark:bg-slate-800"
          >
            <svg class="h-16 w-16 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              />
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
          </div>
        </div>
        <h2 class="mb-2 text-2xl font-semibold text-slate-900 dark:text-white">
          Initializing WhatsApp
        </h2>
        <p class="text-slate-600 dark:text-slate-400">Please wait while we connect...</p>
      </div>
    {:else if qrCode}
      <div class="text-center">
        <h2 class="mb-2 text-2xl font-semibold text-slate-900 dark:text-white">Link your device</h2>
        <p class="mb-6 text-slate-600 dark:text-slate-400">
          Scan this QR code with WhatsApp mobile
        </p>

        <div
          class="mx-auto mb-6 justify-items-center rounded-2xl bg-white p-6 shadow-2xl dark:bg-slate-800"
        >
          <HeadlessQr text={qrCode} size={350}></HeadlessQr>
        </div>

        <div class="space-y-2 text-sm">
          <p class="text-slate-600 dark:text-slate-400">1. Open WhatsApp on your phone</p>
          <p class="text-slate-600 dark:text-slate-400">
            2. Tap <span class="font-semibold">Menu</span> or
            <span class="font-semibold">Settings</span>
          </p>
          <p class="text-slate-600 dark:text-slate-400">
            3. Tap <span class="font-semibold">Linked devices</span> →
            <span class="font-semibold">Link a device</span>
          </p>
        </div>
      </div>
    {:else if status === 'error'}
      <div class="text-center">
        <div class="relative mx-auto mb-8 h-32 w-32">
          <div
            class="relative flex h-full items-center justify-center rounded-full bg-red-50 shadow-xl dark:bg-red-900/20"
          >
            <svg
              class="h-16 w-16 text-red-500"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
        </div>
        <h2 class="mb-2 text-2xl font-semibold text-slate-900 dark:text-white">
          Connection failed
        </h2>
        <p class="mb-4 text-slate-600 dark:text-slate-400">
          {error || 'Something went wrong. Please try again.'}
        </p>
      </div>
    {/if}
  </div>
</div>
