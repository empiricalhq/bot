<script lang="ts">
  import { botStore } from '$lib/stores/bot.store';
  import { AddAllowedUser } from '$lib/wailsjs/go/main/App.js';

  let newUser = $state('');
  let isAddingUser = $state(false);

  async function handleAddUser(e: Event) {
    e.preventDefault();
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
</script>

<div class="mb-6 rounded-xl bg-white p-6 shadow-sm dark:bg-slate-800">
  <h3 class="mb-4 text-lg font-semibold text-slate-900 dark:text-white">Add authorized user</h3>
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
