<script lang="ts">
  import { api, editHref, key, pageHref, type Hit } from '../lib/api'
  import { router } from '../lib/router.svelte'
  import { store } from '../lib/store.svelte'
  import { highlight, plural } from '../lib/format'

  const q = $derived(router.query.get('q') ?? '')
  let hits = $state<Hit[] | null>(null)

  $effect(() => {
    const query = q
    document.title = `Пошук: ${query} — ${store.world?.name}`
    api.search(query).then((h) => {
      if (query === q) hits = h
    })
  })

  const exact = $derived(hits?.some((h) => key(h.title) === key(q)))
</script>

<h1 class="page-title">Результати пошуку</h1>
<p class="muted">
  «{q}»: {hits ? plural(hits.length, ['результат', 'результати', 'результатів']) : '…'}
  {#if hits && !exact && q.trim()}
    · <a class="new" href={editHref(q.trim())}>Створити статтю «{q.trim()}»</a>
  {/if}
</p>

{#if hits}
  <ul class="results">
    {#each hits as hit (hit.title)}
      <li>
        <a href={pageHref(hit.title)}>{hit.title}</a>
        <p>
          {#each highlight(hit.snippet, q) as part, i (i)}{#if part.match}<mark>{part.text}</mark>{:else}{part.text}{/if}{/each}
        </p>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .results {
    list-style: none;
    padding: 0;
    max-width: 48rem;
  }
  .results li {
    margin-bottom: 1rem;
  }
  .results a {
    font-weight: 600;
    font-size: 1.05rem;
  }
  .results p {
    margin: 0.15rem 0 0;
    font-size: 0.9rem;
    color: var(--text-muted);
  }
</style>
