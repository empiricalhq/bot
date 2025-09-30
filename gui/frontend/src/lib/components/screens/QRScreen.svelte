<script lang="ts">
  import { botStore } from '$lib/stores/bot.store';
  import { fade, scale } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';

  $: qrCode = $botStore.qrCode;
  $: status = $botStore.status;
</script>

<div
  class="flex h-full items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-950"
>
  <div class="w-full max-w-md px-6">
    {#if status === 'connecting'}
      <div in:fade={{ duration: 200 }} class="text-center">
        <div class="relative mx-auto mb-8 h-32 w-32">
          <div class="absolute inset-0 animate-ping rounded-full bg-primary/20"></div>
          <div
            class="animation-delay-200 absolute inset-0 animate-ping rounded-full bg-primary/20"
          ></div>
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
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
          </div>
        </div>
        <h2 class="mb-2 text-2xl font-semibold text-slate-900 dark:text-white">
          Initializing WhatsApp
        </h2>
        <p class="text-slate-600 dark:text-slate-400">Please wait while we connect...</p>
      </div>
    {:else if qrCode}
      <div in:scale={{ duration: 300, easing: cubicOut }} class="text-center">
        <div class="mb-8">
          <div class="relative mx-auto h-24 w-24">
            <div class="absolute inset-0 animate-pulse rounded-2xl bg-green-500/20"></div>
            <div
              class="relative flex h-full items-center justify-center rounded-2xl bg-white shadow-xl dark:bg-slate-800"
            >
              <svg class="h-12 w-12 text-green-500" fill="currentColor" viewBox="0 0 24 24">
                <path
                  d="M12.04 2C6.58 2 2.13 6.45 2.13 11.91C2.13 13.66 2.59 15.36 3.45 16.86L2.05 22L7.3 20.62C8.75 21.41 10.38 21.83 12.04 21.83C17.5 21.83 21.95 17.38 21.95 11.92C21.95 9.27 20.92 6.78 19.05 4.91C17.18 3.03 14.69 2 12.04 2M12.05 3.67C14.25 3.67 16.31 4.53 17.87 6.09C19.42 7.65 20.28 9.72 20.28 11.92C20.28 16.46 16.58 20.15 12.04 20.15C10.56 20.15 9.11 19.76 7.85 19L7.55 18.83L4.43 19.65L5.26 16.61L5.06 16.29C4.24 15 3.8 13.47 3.8 11.91C3.81 7.37 7.5 3.67 12.05 3.67M8.53 7.33C8.37 7.33 8.1 7.39 7.87 7.64C7.65 7.89 7 8.5 7 9.71C7 10.93 7.89 12.1 8 12.27C8.14 12.44 9.76 14.94 12.25 16C12.84 16.27 13.3 16.42 13.66 16.53C14.25 16.72 14.79 16.69 15.22 16.63C15.7 16.56 16.68 16.03 16.89 15.45C17.1 14.87 17.1 14.38 17.04 14.27C16.97 14.17 16.81 14.11 16.56 14C16.31 13.86 15.09 13.26 14.87 13.18C14.64 13.1 14.5 13.06 14.31 13.3C14.15 13.55 13.67 14.11 13.53 14.27C13.38 14.44 13.24 14.46 13 14.34C12.74 14.21 11.94 13.95 11 13.11C10.26 12.45 9.77 11.64 9.62 11.39C9.5 11.15 9.61 11 9.73 10.89C9.84 10.78 10 10.6 10.1 10.45C10.23 10.31 10.27 10.2 10.35 10.04C10.43 9.87 10.39 9.73 10.33 9.61C10.27 9.5 9.77 8.26 9.56 7.77C9.36 7.29 9.16 7.35 9 7.34C8.86 7.34 8.7 7.33 8.53 7.33Z"
                />
              </svg>
            </div>
          </div>
          <h2 class="mb-2 text-2xl font-semibold text-slate-900 dark:text-white">
            Link your device
          </h2>
          <p class="text-slate-600 dark:text-slate-400">Scan this QR code with WhatsApp mobile</p>
        </div>

        <div
          class="relative mx-auto mb-6 overflow-hidden rounded-2xl bg-white p-6 shadow-2xl dark:bg-slate-800"
        >
          <div class="absolute -top-4 -left-4 h-8 w-8 border-t-4 border-l-4 border-primary"></div>
          <div class="absolute -top-4 -right-4 h-8 w-8 border-t-4 border-r-4 border-primary"></div>
          <div
            class="absolute -bottom-4 -left-4 h-8 w-8 border-b-4 border-l-4 border-primary"
          ></div>
          <div
            class="absolute -right-4 -bottom-4 h-8 w-8 border-r-4 border-b-4 border-primary"
          ></div>

          <pre
            class="overflow-x-auto font-mono text-[10px] leading-[0.85] text-slate-900 dark:text-slate-100">{qrCode}</pre>
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
      <div in:fade={{ duration: 200 }} class="text-center">
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
          {$botStore.error || 'Something went wrong. Please try again.'}
        </p>
      </div>
    {/if}
  </div>
</div>

<style>
  .animation-delay-200 {
    animation-delay: 200ms;
  }
</style>
