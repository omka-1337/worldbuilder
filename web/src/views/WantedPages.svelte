<script lang="ts">
  import { api, pageHref, type Wanted } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { plural } from '../lib/format'

  let wanted = $state<Wanted[] | null>(null)
  api.wanted().then((w) => (wanted = w))
  document.title = `Потрібні статті — ${store.world?.name}`
</script>

<h1 class="page-title">Потрібні статті</h1>
<p class="muted">Статті, на які вже є посилання, але які ще не написані. Вгорі ті, на які посилаються найчастіше.</p>

{#if wanted?.length === 0}
  <p>Червоних посилань немає. Світ описано повністю (поки що).</p>
{:else if wanted}
  <ol>
    {#each wanted as w (w.title)}
      <li>
        <a class="new" href={pageHref(w.title)}>{w.title}</a>
        <span class="muted">
          — {plural(w.from.length, ['посилання', 'посилання', 'посилань'])}:
          {#each w.from as from, i (from)}{i ? ', ' : ''}<a href={pageHref(from)}>{from}</a>{/each}
        </span>
      </li>
    {/each}
  </ol>
{/if}

<style>
  li {
    margin: 0.3rem 0;
  }
</style>
