<script lang="ts">
  import '../app.css';

  import { onMount } from 'svelte';
  import { botStore } from '$lib/stores/bot.store.svelte';
  import ConnectionStatusScreen from '$lib/components/connection/connection-status.svelte';
  import DashboardScreen from '$lib/components/dashboard/dashboard.svelte';
  import { initialize } from '$lib/services/bot.service';

  onMount(initialize);

  const { status } = $derived(botStore);
</script>

<svelte:head>
  <title>whatsbot</title>
</svelte:head>

<div class="h-screen overflow-hidden">
  {#if status === 'connected'}
    <DashboardScreen />
  {:else}
    <ConnectionStatusScreen />
  {/if}
</div>
