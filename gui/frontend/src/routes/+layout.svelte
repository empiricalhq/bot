<script lang="ts">
  import Header from './Header.svelte';
  import '../app.css';

  let { children } = $props();
</script>

<div class="flex min-h-screen flex-col">
  <Header />

  <main>
    {@render children()}
  </main>

  <footer class="border-t py-4 text-center text-sm text-muted-foreground">
    <p>
      visit <a href="https://svelte.dev/docs/kit">svelte.dev/docs/kit</a> to learn about SvelteKit
    </p>
  </footer>
</div>
