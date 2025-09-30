<script lang="ts">
  import { botStore } from '$lib/stores/bot.store';

  const allowedUsers = $derived(Array.from($botStore.allowedUsers));
</script>

<div class="rounded-xl bg-white shadow-sm dark:bg-slate-800">
  <div class="border-b px-6 py-4 dark:border-slate-700">
    <h3 class="text-lg font-semibold text-slate-900 dark:text-white">Authorized users</h3>
  </div>
  <div class="max-h-96 overflow-y-auto">
    {#if allowedUsers.length > 0}
      {#each allowedUsers as user (user)}
        <div
          class="flex items-center justify-between border-b px-6 py-3 transition-colors last:border-0 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-700/50"
        >
          <div class="flex items-center gap-3">
            <div
              class="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-sm font-medium text-primary"
            >
              {user.charAt(0).toUpperCase()}
            </div>
            <span class="font-medium text-slate-900 dark:text-white">{user}</span>
          </div>
        </div>
      {/each}
    {:else}
      <div class="px-6 py-12 text-center">
        <p class="text-slate-500 dark:text-slate-400">No authorized users yet</p>
        <p class="mt-1 text-sm text-slate-400 dark:text-slate-500">
          Add a phone number above to get started
        </p>
      </div>
    {/if}
  </div>
</div>
