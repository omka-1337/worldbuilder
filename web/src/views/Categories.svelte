<script lang="ts">
  import { api, categoryHref, type Category } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { byLetter } from '../lib/format'

  let categories = $state<Category[] | null>(null)
  api.categories().then((c) => (categories = c))
  document.title = `Категорії — ${store.world?.name}`
</script>

<h1 class="page-title">Категорії</h1>
{#if categories?.length === 0}
  <p class="muted">Категорій ще немає. Додайте <code>categories: [Назва]</code> у frontmatter статті.</p>
{:else if categories}
  <div class="letter-groups">
    {#each byLetter(categories, (c) => c.name) as [letter, items] (letter)}
      <section>
        <h3>{letter}</h3>
        <ul>
          {#each items as c (c.name)}
            <li><a href={categoryHref(c.name)}>{c.name}</a> <span class="muted">({c.count})</span></li>
          {/each}
        </ul>
      </section>
    {/each}
  </div>
{/if}
