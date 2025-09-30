<script lang="ts">
  import { botStore, type LogEntry } from '$lib/stores/bot.store';
  import { slide, fade } from 'svelte/transition';
  import { AddAllowedUser } from '$lib/wailsjs/go/main/App.js';

  let newUser = '';
  let isAddingUser = false;
  let activeTab: 'users' | 'messages' | 'logs' = 'users';

  $: allowedUsers = Array.from($botStore.allowedUsers);
  $: messages = $botStore.messages;
  $: logs = $botStore.logs;

  async function handleAddUser() {
    if (!newUser.trim() || isAddingUser) return;

    const formattedUser = newUser.includes('@') ? newUser : `${newUser}@s.whatsapp.net`;
    isAddingUser = true;

    try {
      await AddAllowedUser(formattedUser);
      botStore.addAllowedUser(formattedUser);
      newUser = '';
    } finally {
      isAddingUser = false;
    }
  }

  function getLogColor(level: LogEntry['level']) {
    switch (level) {
      case 'ERROR':
        return 'text-red-500';
      case 'WARN':
        return 'text-yellow-500';
      case 'INFO':
        return 'text-blue-500';
      default:
        return 'text-slate-500';
    }
  }

  function formatTime(date: Date) {
    return date.toLocaleTimeString('en-US', { hour12: false });
  }
</script>

<div class="flex h-full flex-col bg-slate-50 dark:bg-slate-900">
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
        class="relative px-4 py-3 text-sm font-medium transition-colors"
        class:text-primary={activeTab === 'users'}
        class:text-slate-600={activeTab !== 'users'}
        class:dark:text-slate-400={activeTab !== 'users'}
      >
        Authorized users
        {#if activeTab === 'users'}
          <div
            transition:slide={{ duration: 200 }}
            class="absolute right-0 bottom-0 left-0 h-0.5 bg-primary"
          ></div>
        {/if}
      </button>

      <button
        onclick={() => (activeTab = 'messages')}
        class="relative px-4 py-3 text-sm font-medium transition-colors"
        class:text-primary={activeTab === 'messages'}
        class:text-slate-600={activeTab !== 'messages'}
        class:dark:text-slate-400={activeTab !== 'messages'}
      >
        Messages
        {#if messages.length > 0}
          <span class="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
            {messages.length}
          </span>
        {/if}
        {#if activeTab === 'messages'}
          <div
            transition:slide={{ duration: 200 }}
            class="absolute right-0 bottom-0 left-0 h-0.5 bg-primary"
          ></div>
        {/if}
      </button>

      <button
        onclick={() => (activeTab = 'logs')}
        class="relative px-4 py-3 text-sm font-medium transition-colors"
        class:text-primary={activeTab === 'logs'}
        class:text-slate-600={activeTab !== 'logs'}
        class:dark:text-slate-400={activeTab !== 'logs'}
      >
        Activity logs
        {#if activeTab === 'logs'}
          <div
            transition:slide={{ duration: 200 }}
            class="absolute right-0 bottom-0 left-0 h-0.5 bg-primary"
          ></div>
        {/if}
      </button>
    </nav>
  </div>

  <div class="flex-1 overflow-hidden">
    {#if activeTab === 'users'}
      <div class="h-full p-6">
        <div class="mx-auto max-w-2xl">
          <div class="mb-6 rounded-xl bg-white p-6 shadow-sm dark:bg-slate-800">
            <h3 class="mb-4 text-lg font-semibold text-slate-900 dark:text-white">
              Add authorized user
            </h3>
            <form onsubmit={handleAddUser} class="flex gap-3">
              <input
                bind:value={newUser}
                type="text"
                placeholder="Phone number (e.g., 1234567890)"
                class="flex-1 rounded-lg border bg-slate-50 px-4 py-2.5 text-slate-900 placeholder-slate-500 transition-colors focus:border-primary focus:bg-white focus:outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-white dark:focus:bg-slate-950"
                disabled={isAddingUser}
              />
              <button
                type="submit"
                disabled={isAddingUser || !newUser.trim()}
                class="rounded-lg bg-primary px-6 py-2.5 font-medium text-white transition-all hover:bg-primary/90 disabled:opacity-50"
              >
                {isAddingUser ? 'Adding...' : 'Add User'}
              </button>
            </form>
            <p class="mt-2 text-sm text-slate-600 dark:text-slate-400">
              Enter phone number without country code or special characters
            </p>
          </div>

          <!-- Users List -->
          <div class="rounded-xl bg-white shadow-sm dark:bg-slate-800">
            <div class="border-b px-6 py-4 dark:border-slate-700">
              <h3 class="text-lg font-semibold text-slate-900 dark:text-white">Authorized users</h3>
            </div>
            <div class="max-h-96 overflow-y-auto">
              {#if allowedUsers.length > 0}
                {#each allowedUsers as user, i}
                  <div
                    in:fade={{ duration: 200, delay: i * 50 }}
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
        </div>
      </div>
    {:else if activeTab === 'messages'}
      <div class="flex h-full flex-col">
        <div class="flex-1 overflow-y-auto px-6 py-4">
          {#if messages.length > 0}
            <div class="space-y-3">
              {#each messages as message}
                <div
                  in:fade={{ duration: 200 }}
                  class="flex"
                  class:justify-end={message.direction === 'outgoing'}
                >
                  <div
                    class="max-w-lg rounded-2xl px-4 py-3 shadow-sm"
                    class:bg-primary={message.direction === 'outgoing'}
                    class:text-white={message.direction === 'outgoing'}
                    class:bg-white={message.direction === 'incoming'}
                    class:dark:bg-slate-800={message.direction === 'incoming'}
                    class:text-slate-900={message.direction === 'incoming'}
                    class:dark:text-white={message.direction === 'incoming'}
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
      </div>
    {:else if activeTab === 'logs'}
      <div class="h-full overflow-y-auto bg-slate-900 p-4 font-mono text-xs">
        {#if logs.length > 0}
          <div class="space-y-1">
            {#each logs as log}
              <div in:fade={{ duration: 100 }} class="flex gap-3">
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
    {/if}
  </div>
</div>
