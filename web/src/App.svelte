<script lang="ts">
  import { router, interceptLinks } from './lib/router.svelte'
  import Home from './views/Home.svelte'
  import WorldShell from './components/WorldShell.svelte'

  // "/" is the world picker; "/w/{id}/…" is inside a world.
  const route = $derived.by(() => {
    const [, first = '', id = '', ...rest] = router.path.split('/')
    if (first === '') return { name: 'home' as const }
    if (first === 'w' && id) return { name: 'world' as const, id: decodeURIComponent(id), rest }
    return { name: 'unknown' as const }
  })
</script>

<svelte:document onclick={interceptLinks} />

{#if route.name === 'home'}
  <Home />
{:else if route.name === 'world'}
  {#key route.id}<WorldShell id={route.id} rest={route.rest} />{/key}
{:else}
  <main class="standalone">
    <h1 class="page-title">Сторінку не знайдено</h1>
    <p><a href="/">До списку світів</a></p>
  </main>
{/if}
