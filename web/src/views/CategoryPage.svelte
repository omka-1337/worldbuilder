<script lang="ts">
  import { api, pageHref, worldHref, type Summary } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { byLetter, plural } from '../lib/format'

  let { name }: { name: string } = $props()

  let pages = $state<Summary[] | null>(null)
  $effect(() => {
    api.category(name).then((p) => (pages = p))
    document.title = `Категорія: ${name} — ${store.world?.name}`
  })
</script>

<p class="muted kicker"><a href={worldHref(undefined, '/categories')}>Категорії</a></p>
<h1 class="page-title">{name}</h1>
{#if pages}
  <p class="muted">{plural(pages.length, ['стаття', 'статті', 'статей'])} у цій категорії</p>
  <div class="letter-groups">
    {#each byLetter(pages, (p) => p.title) as [letter, items] (letter)}
      <section>
        <h3>{letter}</h3>
        <ul>
          {#each items as p (p.title)}<li><a href={pageHref(p.title)}>{p.title}</a></li>{/each}
        </ul>
      </section>
    {/each}
  </div>
{/if}

<style>
  .kicker {
    margin: 0;
    font-size: 0.85rem;
  }
</style>
