<script lang="ts">
  import { botStore } from '$lib/stores/bot.store.svelte';

  const { messages } = $derived(botStore);

  function formatTime(date: Date): string {
    return date.toLocaleTimeString('en-US', { hour12: false });
  }
</script>

<div class="flex h-full flex-col overflow-y-auto px-6 py-4">
  {#if messages.length > 0}
    <div class="space-y-3">
      {#each messages as message (message.id)}
        <div class="flex {message.direction === 'outgoing' ? 'justify-end' : ''}">
          <div
            class="max-w-lg rounded-2xl px-4 py-3 shadow-sm {message.direction === 'outgoing'
              ? 'bg-primary text-white'
              : 'bg-white text-slate-900 dark:bg-slate-800 dark:text-white'}"
          >
            <div class="mb-1 flex items-center gap-2 text-xs opacity-75">
              <span class="font-medium">{message.userName}</span>
              <span>{formatTime(message.timestamp)}</span>
            </div>
            <p class="text-sm">{message.text}</p>
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <div class="flex h-full items-center justify-center">
      <div class="text-center">
        <svg
          class="mx-auto mb-4 h-16 w-16 text-slate-300 dark:text-slate-700"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
          />
        </svg>
        <p class="text-slate-500 dark:text-slate-400">No messages yet</p>
        <p class="mt-1 text-sm text-slate-400 dark:text-slate-500">
          Messages will appear here when users interact with the bot
        </p>
      </div>
    </div>
  {/if}
</div>
