<script lang="ts">
  import { setWorld, NotFoundError } from '../lib/api'
  import { router, titleFrom } from '../lib/router.svelte'
  import { store, refresh, reset } from '../lib/store.svelte'
  import Sidebar from './Sidebar.svelte'
  import Article from '../views/Article.svelte'
  import Editor from '../views/Editor.svelte'
  import AllPages from '../views/AllPages.svelte'
  import Categories from '../views/Categories.svelte'
  import CategoryPage from '../views/CategoryPage.svelte'
  import WantedPages from '../views/WantedPages.svelte'
  import SearchResults from '../views/SearchResults.svelte'
  import NewPage from '../views/NewPage.svelte'

  // Everything inside a world is at /w/{id}/…; `rest` is the part after it.
  let { id, rest }: { id: string; rest: string[] } = $props()

  // Runs before children render, so their API calls and links use this world.
  // svelte-ignore state_referenced_locally
  setWorld(id)
  reset()

  let loadError = $state('')
  let menuOpen = $state(false)

  refresh().catch((e) => (loadError = e instanceof NotFoundError ? 'missing' : String(e)))

  type Route =
    | { name: 'article' | 'edit' | 'category'; arg: string }
    | { name: 'pages' | 'categories' | 'wanted' | 'search' | 'new' | 'unknown' }

  const route: Route = $derived.by(() => {
    const [first = '', ...more] = rest
    const arg = titleFrom(more.join('/'))
    switch (first) {
      case '':
        return { name: 'article', arg: store.world?.mainPage ?? '' }
      case 'wiki':
        return { name: 'article', arg }
      case 'edit':
        return { name: 'edit', arg }
      case 'category':
        return { name: 'category', arg }
      case 'pages':
      case 'categories':
      case 'wanted':
      case 'search':
      case 'new':
        return { name: first }
    }
    return { name: 'unknown' }
  })

  // Close the mobile menu after navigating.
  $effect(() => {
    void router.path
    menuOpen = false
  })
</script>

{#if loadError === 'missing'}
  <main class="standalone">
    <h1 class="page-title">Світу «{id}» немає</h1>
    <p><a href="/">До списку світів</a></p>
  </main>
{:else}
  <div class="layout" class:menu-open={menuOpen}>
    <button class="menu-toggle" onclick={() => (menuOpen = !menuOpen)} aria-label="Меню">☰</button>
    <Sidebar />
    <main>
      {#if loadError}
        <p class="error">Не вдалося з'єднатися з сервером: {loadError}</p>
      {:else if !store.world}
        <p class="muted">Завантаження…</p>
      {:else if route.name === 'article'}
        {#key route.arg}<Article title={route.arg} />{/key}
      {:else if route.name === 'edit'}
        {#key route.arg}<Editor title={route.arg} />{/key}
      {:else if route.name === 'category'}
        {#key route.arg}<CategoryPage name={route.arg} />{/key}
      {:else if route.name === 'pages'}
        <AllPages />
      {:else if route.name === 'categories'}
        <Categories />
      {:else if route.name === 'wanted'}
        <WantedPages />
      {:else if route.name === 'search'}
        <SearchResults />
      {:else if route.name === 'new'}
        <NewPage />
      {:else}
        <h1 class="page-title">Сторінку не знайдено</h1>
      {/if}
    </main>
  </div>
{/if}

<style>
  .layout {
    display: grid;
    grid-template-columns: 17rem minmax(0, 1fr);
    min-height: 100vh;
  }
  main {
    padding: 2rem clamp(1rem, 4vw, 3.5rem) 4rem;
    min-width: 0;
  }
  .error {
    color: var(--danger);
  }
  .menu-toggle {
    display: none;
  }

  @media (max-width: 800px) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }
    main {
      padding: 3.5rem 1rem 3rem;
    }
    .menu-toggle {
      display: block;
      position: fixed;
      top: 0.6rem;
      left: 0.6rem;
      z-index: 20;
    }
    .layout :global(.sidebar) {
      display: none;
    }
    .layout.menu-open :global(.sidebar) {
      display: flex;
      position: fixed;
      inset: 0;
      z-index: 10;
      padding-top: 3.5rem;
    }
  }
</style>
