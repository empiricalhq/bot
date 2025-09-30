<script lang="ts">
  import { botStore } from '$lib/stores/bot.store.svelte';
  import AddUserForm from './add-user-form.svelte';
  import LogView from './log-view.svelte';
  import MessageView from './message-view.svelte';
  import UserList from './user-list.svelte';

  let activeTab = $state<'users' | 'messages' | 'logs'>('users');

  const { messages, allowedUsers } = $derived(botStore);
</script>

<div class="flex h-screen flex-col bg-slate-50 dark:bg-slate-900">
  <div
    class="flex items-center justify-between border-b bg-white px-6 py-3 shadow-sm dark:border-slate-800 dark:bg-slate-950"
  >
    <div class="flex items-center gap-3">
      <div class="relative">
        <div class="h-3 w-3 rounded-full bg-green-500"></div>
        <div class="absolute inset-0 animate-ping rounded-full bg-green-500 opacity-75"></div>
      </div>
      <span class="font-medium text-slate-900 dark:text-white">WhatsBot connected</span>
    </div>
    <div class="text-sm text-slate-600 dark:text-slate-400">
      {allowedUsers.length} authorized user{allowedUsers.length !== 1 ? 's' : ''}
    </div>
  </div>

  <div class="border-b bg-white dark:border-slate-800 dark:bg-slate-950">
    <nav class="flex px-6">
      <button
        onclick={() => (activeTab = 'users')}
        class="relative px-4 py-3 text-sm font-medium transition-colors {activeTab === 'users'
          ? 'text-primary'
          : 'text-slate-600 dark:text-slate-400'}"
      >
        Authorized users
        {#if activeTab === 'users'}
          <div class="absolute right-0 bottom-0 left-0 h-0.5 bg-primary"></div>
        {/if}
      </button>

      <button
        onclick={() => (activeTab = 'messages')}
        class="relative px-4 py-3 text-sm font-medium transition-colors {activeTab === 'messages'
          ? 'text-primary'
          : 'text-slate-600 dark:text-slate-400'}"
      >
        Messages
        {#if messages.length > 0}
          <span class="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
            {messages.length}
          </span>
        {/if}
        {#if activeTab === 'messages'}
          <div class="absolute right-0 bottom-0 left-0 h-0.5 bg-primary"></div>
        {/if}
      </button>

      <button
        onclick={() => (activeTab = 'logs')}
        class="relative px-4 py-3 text-sm font-medium transition-colors {activeTab === 'logs'
          ? 'text-primary'
          : 'text-slate-600 dark:text-slate-400'}"
      >
        Activity logs
        {#if activeTab === 'logs'}
          <div class="absolute right-0 bottom-0 left-0 h-0.5 bg-primary"></div>
        {/if}
      </button>
    </nav>
  </div>

  <div class="flex-1 overflow-hidden">
    {#if activeTab === 'users'}
      <div class="h-full overflow-y-auto p-6">
        <div class="mx-auto max-w-2xl">
          <AddUserForm />
          <UserList />
        </div>
      </div>
    {:else if activeTab === 'messages'}
      <MessageView />
    {:else if activeTab === 'logs'}
      <LogView />
    {/if}
  </div>
</div>
