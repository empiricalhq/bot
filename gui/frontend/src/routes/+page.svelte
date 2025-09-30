<script lang="ts">
  import '../app.css';

  import { onMount } from 'svelte';
  import { isConnected } from '$lib/stores/bot.store';
  import ConnectionStatusScreen from '$lib/components/connection/connection-status.svelte';
  import DashboardScreen from '$lib/components/dashboard/dashboard.svelte';
  import { initialize } from '$lib/services/bot.service';

  onMount(initialize);
</script>

<svelte:head>
  <title>whatsbot</title>
</svelte:head>

<div class="h-screen overflow-hidden">
  {#if $isConnected}
    <DashboardScreen />
  {:else}
    <ConnectionStatusScreen />
  {/if}
</div>
