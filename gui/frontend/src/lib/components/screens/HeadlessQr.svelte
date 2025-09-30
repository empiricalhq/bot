<script lang="ts">
  import { qr } from 'headless-qr';

  export let text: string;
  export let size: number = 250;

  $: modules = text ? qr(text) : [];
</script>

{#if text && modules.length}
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox={`0 0 ${modules.length} ${modules.length}`}
    width={size}
    height={size}
    shape-rendering="crispEdges"
  >
    {#each modules as row, y (y)}
      {#each row as cell, x (x)}
        {#if cell}
          <rect {x} {y} width="1" height="1" fill="currentColor" />
        {/if}
      {/each}
    {/each}
  </svg>
{:else}
  <slot>No QR</slot>
{/if}
