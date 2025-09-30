<script lang="ts">
  import { botStore } from '$lib/stores/bot.store.svelte';
  import { getLogColor, formatTime } from '$lib/utils/formatters';

  const { logs } = $derived(botStore);
</script>

<div class="h-full overflow-y-auto bg-slate-900 p-4 font-mono text-xs">
  {#if logs.length > 0}
    <div class="space-y-1">
      {#each logs as log (log.id)}
        <div class="flex gap-3">
          <span class="text-slate-600">{formatTime(log.timestamp)}</span>
          <span class={getLogColor(log.level)}>[{log.level}]</span>
          <span class="text-slate-300">{log.message}</span>
        </div>
      {/each}
    </div>
  {:else}
    <div class="flex h-full items-center justify-center">
      <p class="text-slate-600">Waiting for activity...</p>
    </div>
  {/if}
</div>
