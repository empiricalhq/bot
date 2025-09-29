<script lang="ts">
  import { Button } from '$lib/components/ui/button/index.js';
</script>

<svelte:head>
  <title>Home</title>
  <meta name="description" content="Svelte demo app" />
</svelte:head>

<section
  class="flex h-full min-h-[70vh] flex-col items-center justify-center space-y-6 text-center"
>
  <h1 class="text-4xl font-extrabold tracking-tight lg:text-5xl">SvelteKit + Shadcn</h1>
  <p class="max-w-xl text-lg text-muted-foreground">
    Idk what i'm going to put here, but oh god, hopefully it looks better than this.
  </p>

  <Button>Click me</Button>
</section>
