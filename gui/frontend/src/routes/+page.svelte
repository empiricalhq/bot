<svelte:head>
  <title>Home</title>
  <meta name="description" content="Svelte demo app" />
</svelte:head>

<script lang="ts">
  import { Button } from "$lib/components/ui/button/index.js";
</script>

<section class="flex flex-col items-center justify-center h-full min-h-[70vh] text-center space-y-6">
  <h1 class="text-4xl font-extrabold tracking-tight lg:text-5xl">
   SvelteKit + Shadcn
  </h1>
  <p class="text-lg text-muted-foreground max-w-xl">
    Idk what i'm going to put here, but oh god, hopefully it looks better than this.
  </p>
  
  <Button>Click me</Button>
</section>
