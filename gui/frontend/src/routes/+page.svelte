<script lang="ts">
  import { onMount } from 'svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { EventsOn } from '$lib/wailsjs/runtime/runtime.js';
  import { StartBot, GetAllowedUsers, AddAllowedUser } from '$lib/wailsjs/go/main/App.js';

  let botStatus: 'Stopped' | 'Starting' | 'Waiting for Scan' | 'Running' | 'Failed' = 'Stopped';
  let qrCode = '';
  let allowedUsers: string[] = [];
  let newUser = '';
  let errorMessage = '';

  onMount(() => {
    refreshAllowedUsers();

    EventsOn('bot:qr_code', (code) => {
      qrCode = code;
      botStatus = 'Waiting for Scan';
    });

    EventsOn('bot:new_log', (log: { level: string; message: string }) => {
      if (
        log.message.includes('QR login successful') ||
        log.message.includes('Connection successful')
      ) {
        qrCode = '';
        botStatus = 'Running';
      }
    });

    EventsOn('bot:start_failed', (err) => {
      botStatus = 'Failed';
      errorMessage = err;
    });
  });

  async function handleStartBot() {
    botStatus = 'Starting';
    errorMessage = '';
    qrCode = '';
    await StartBot();
  }

  async function handleAddUser() {
    if (!newUser.trim()) return;
    await AddAllowedUser(newUser);
    newUser = '';
    await refreshAllowedUsers();
  }

  async function refreshAllowedUsers() {
    allowedUsers = await GetAllowedUsers();
  }
</script>

<svelte:head>
  <title>WhatsBot Showcase</title>
  <meta name="description" content="A GUI to demonstrate the WhatsBot functionality." />
</svelte:head>

<section class="container mx-auto grid grid-cols-1 gap-8 px-4 py-8 md:grid-cols-2">
  <div class="flex flex-col space-y-6 rounded-lg border bg-card p-6 text-card-foreground">
    <h2 class="text-2xl font-bold">Controls & Status</h2>

    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold">Bot Status</h3>
        <span
          class="rounded-full px-3 py-1 text-sm font-medium"
          class:bg-red-200={botStatus === 'Stopped' || botStatus === 'Failed'}
          class:text-red-800={botStatus === 'Stopped' || botStatus === 'Failed'}
          class:bg-yellow-200={botStatus === 'Starting' || botStatus === 'Waiting for Scan'}
          class:text-yellow-800={botStatus === 'Starting' || botStatus === 'Waiting for Scan'}
          class:bg-green-200={botStatus === 'Running'}
          class:text-green-800={botStatus === 'Running'}
        >
          {botStatus}
        </span>
      </div>
      <Button onclick={handleStartBot} disabled={botStatus !== 'Stopped' && botStatus !== 'Failed'}>
        {#if botStatus === 'Starting' || botStatus === 'Waiting for Scan'}
          Starting...
        {:else if botStatus === 'Running'}
          Running
        {:else}
          Start Bot
        {/if}
      </Button>
      {#if botStatus === 'Failed'}
        <p class="text-sm text-red-500">Failed to start: {errorMessage}</p>
      {/if}
    </div>

    {#if qrCode}
      <div class="space-y-2">
        <h3 class="font-semibold">Scan QR Code</h3>
        <p class="text-sm text-muted-foreground">
          Open WhatsApp on your phone and link this device.
        </p>
        <div class="overflow-x-auto rounded-md bg-muted p-4">
          <pre class="font-mono text-xs text-muted-foreground">{qrCode}</pre>
        </div>
      </div>
    {/if}

    <div class="space-y-2">
      <h3 class="font-semibold">Allowed users (dev mode)</h3>
      <p class="text-sm text-muted-foreground">
        Add phone numbers (e.g., 1234567890@s.whatsapp.net) to interact with the bot.
      </p>
      <ul class="max-h-32 overflow-y-auto rounded-md border p-2 text-sm">
        {#if allowedUsers.length > 0}
          {#each allowedUsers as user}
            <li class="px-2 py-1">{user}</li>
          {/each}
        {:else}
          <li class="px-2 py-1 text-muted-foreground">No users specified.</li>
        {/if}
      </ul>
      <form on:submit|preventDefault={handleAddUser} class="flex items-center space-x-2">
        <Input bind:value={newUser} placeholder="user@s.whatsapp.net" class="flex-grow" />
        <Button type="submit">Add</Button>
      </form>
    </div>
  </div>

  <div class="flex flex-col space-y-6">
    <div class="flex h-80 flex-col space-y-2 rounded-lg border bg-card p-6 text-card-foreground">
      <h3 class="text-xl font-bold">Incoming Messages</h3>
      <div class="flex-grow overflow-y-auto rounded-md bg-muted p-4">
        <p class="text-center text-muted-foreground">Waiting for messages...</p>
      </div>
    </div>

    <div class="flex h-80 flex-col space-y-2 rounded-lg border bg-card p-6 text-card-foreground">
      <h3 class="text-xl font-bold">Bot Activity Log</h3>
      <div class="flex-grow overflow-y-auto rounded-md bg-muted p-4">
        <p class="text-center text-muted-foreground">Waiting for logs...</p>
      </div>
    </div>
  </div>
</section>
