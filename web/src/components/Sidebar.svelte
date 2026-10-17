<script lang="ts">
  import Icon from './Icon.svelte'
  import SettingsDialog from './SettingsDialog.svelte'
  import { api, key, pageHref, worldHref, type Hit } from '../lib/api'
  import { router, navigate, titleFrom } from '../lib/router.svelte'
  import { store } from '../lib/store.svelte'

  let query = $state('')
  let hits = $state<Hit[]>([])
  let settingsOpen = $state(false)
  let timer: ReturnType<typeof setTimeout>

  function onInput() {
    clearTimeout(timer)
    const q = query.trim()
    if (!q) {
      hits = []
      return
    }
    timer = setTimeout(async () => {
      const found = await api.search(q, 8)
      if (query.trim() === q) hits = found
    }, 120)
  }

  // Enter goes straight to an exactly matching article, otherwise to results.
  async function onSubmit(e: SubmitEvent) {
    e.preventDefault()
    const q = query.trim()
    if (!q) return
    const found = await api.search(q, 1)
    if (found[0] && key(found[0].title) === key(q)) navigate(pageHref(found[0].title))
    else navigate(worldHref(undefined, '/search?q=' + encodeURIComponent(q)))
    reset()
  }

  function reset() {
    query = ''
    hits = []
  }

  // The article open in the main pane, to highlight it in the list.
  const current = $derived.by(() => {
    const prefix = new URL(worldHref(undefined, '/wiki/'), location.href).pathname
    return router.path.startsWith(prefix) ? key(titleFrom(router.path.slice(prefix.length))) : ''
  })
</script>

<aside class="sidebar">
  <form class="search" onsubmit={onSubmit}>
    <input
      type="search"
      placeholder="Пошук статей…"
      aria-label="Пошук статей"
      bind:value={query}
      oninput={onInput}
      onkeydown={(e) => e.key === 'Escape' && reset()}
    />
    {#if hits.length}
      <ul class="suggestions">
        {#each hits as hit (hit.title)}
          <li><a href={pageHref(hit.title)} onclick={reset}>{hit.title}</a></li>
        {/each}
      </ul>
    {/if}
  </form>

  <h2>Статті <span class="muted">{store.pages.length}</span></h2>
  <ul class="pages">
    {#each store.pages as page (page.title)}
      <li>
        <a href={pageHref(page.title)} class:active={current === key(page.title)}>{page.title}</a>
      </li>
    {/each}
  </ul>

  <footer>
    <a class="world" href={worldHref()} title="Головна сторінка світу">{store.world?.name ?? ''}</a>
    <button class="icon" onclick={() => (settingsOpen = true)} title="Налаштування" aria-label="Налаштування">
      <Icon name="settings" size={18} />
    </button>
    <a class="icon" href="/" title="Вийти зі світу" aria-label="Вийти зі світу">
      <Icon name="exit" size={18} />
    </a>
  </footer>
</aside>

<SettingsDialog bind:open={settingsOpen} />

<style>
  .sidebar {
    position: sticky;
    top: 0;
    height: 100vh;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    padding: 0.75rem 0.75rem 0;
    background: var(--bg-sidebar);
    border-inline: 1px solid var(--border);
    overflow: hidden;
  }
  .search {
    position: relative;
  }
  .search input {
    width: 100%;
    padding: 0.4rem 0.65rem;
  }
  .suggestions {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    z-index: 5;
    list-style: none;
    margin: 0;
    padding: 0.25rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.12);
  }
  .suggestions a {
    display: block;
    padding: 0.3rem 0.5rem;
    border-radius: 4px;
    color: var(--text);
  }
  .suggestions a:hover {
    background: var(--bg-hover);
    text-decoration: none;
  }
  h2 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-muted);
    margin: 0.5rem 0.5rem 0;
  }
  .pages {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    flex: 1;
    font-size: 0.93rem;
  }
  .pages a {
    display: block;
    padding: 0.2rem 0.5rem;
    border-radius: 5px;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .pages a:hover {
    background: var(--bg-hover);
    text-decoration: none;
  }
  .pages a.active {
    background: var(--accent-soft);
    color: var(--accent);
  }

  footer {
    display: flex;
    align-items: center;
    gap: 0.15rem;
    margin: 0 -0.75rem;
    padding: 0.55rem 0.6rem 0.55rem 1.25rem;
    border-top: 1px solid var(--border);
  }
  .world {
    flex: 1;
    min-width: 0;
    font-weight: 700;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .icon {
    display: grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    padding: 0;
    border: none;
    border-radius: 6px;
    background: none;
    color: var(--text-muted);
  }
  .icon:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
</style>
