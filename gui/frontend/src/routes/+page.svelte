<script lang="ts">
  import '../app.css';

  import { onMount } from 'svelte';
  import { botStore, isConnected } from '$lib/stores/bot.store';
  import QRScreen from '$lib/components/screens/QRScreen.svelte';
  import DashboardScreen from '$lib/components/screens/DashboardScreen.svelte';
  import { EventsOn } from '$lib/wailsjs/runtime/runtime.js';
  import { StartBot, GetAllowedUsers } from '$lib/wailsjs/go/main/App.js';

  let isInitialized = false;

  onMount(() => {
    setupEventListeners();
    initializeBot();
  });

  function setupEventListeners() {
    EventsOn('bot:qr_code', (qrCode: string) => {
      botStore.setQRCode(qrCode);
    });

    EventsOn('bot:new_log', (log: { level: string; message: string }) => {
      const level = log.level.toUpperCase() as 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
      botStore.addLog({ level, message: log.message });

      if (
        log.message.includes('QR login successful') ||
        log.message.includes('Connection successful')
      ) {
        botStore.clearQRCode();
        botStore.setStatus('connected');
      }
    });

    EventsOn(
      'bot:new_message',
      (message: { direction: string; userID: string; userName: string; text: string }) => {
        botStore.addMessage({
          direction: message.direction as 'incoming' | 'outgoing',
          userId: message.userID,
          userName: message.userName,
          text: message.text
        });
      }
    );

    EventsOn('bot:start_failed', (error: string) => {
      botStore.setError(error);
    });
  }

  async function initializeBot() {
    if (isInitialized) return;
    isInitialized = true;

    botStore.setStatus('connecting');

    try {
      const users = await GetAllowedUsers();
      botStore.setAllowedUsers(users);
      await StartBot();
    } catch (error) {
      console.error('Failed to initialize bot:', error);
      botStore.setError('Failed to initialize bot');
    }
  }
</script>

<svelte:head>
  <title>whatsbot</title>
</svelte:head>

<div class="h-screen overflow-hidden">
  {#if $isConnected}
    <DashboardScreen />
  {:else}
    <QRScreen />
  {/if}
</div>
