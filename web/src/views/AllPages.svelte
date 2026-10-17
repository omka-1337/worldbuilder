<script lang="ts">
  import { pageHref } from '../lib/api'
  import { store, typeLabel } from '../lib/store.svelte'
  import { byLetter, plural } from '../lib/format'

  const groups = $derived(byLetter(store.pages, (p) => p.title))
  document.title = `Усі статті — ${store.world?.name}`
</script>

<h1 class="page-title">Усі статті</h1>
<p class="muted">{plural(store.pages.length, ['стаття', 'статті', 'статей'])}</p>

<div class="letter-groups">
  {#each groups as [letter, pages] (letter)}
    <section>
      <h3>{letter}</h3>
      <ul>
        {#each pages as p (p.title)}
          <li>
            <a href={pageHref(p.title)}>{p.title}</a>
            {#if typeLabel(p.type)}<span class="muted type">{typeLabel(p.type)}</span>{/if}
          </li>
        {/each}
      </ul>
    </section>
  {/each}
</div>

<style>
  .type {
    font-size: 0.8rem;
    margin-left: 0.3rem;
  }
</style>
