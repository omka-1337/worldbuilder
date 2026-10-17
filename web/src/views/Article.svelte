<script lang="ts">
  import { tick } from 'svelte'
  import { api, key, NotFoundError, pageHref, editHref, categoryHref, type PageView, type Wanted } from '../lib/api'
  import { store } from '../lib/store.svelte'

  let { title }: { title: string } = $props()

  let view = $state<PageView | null>(null)
  let missing = $state<Wanted | null>(null)
  let error = $state('')

  $effect(() => {
    load(title)
  })

  async function load(t: string) {
    try {
      view = await api.page(t)
      document.title = `${view.title} — ${store.world?.name}`
      await tick()
      scrollToHash()
    } catch (e) {
      if (!(e instanceof NotFoundError)) {
        error = String(e)
        return
      }
      document.title = `${t} — ${store.world?.name}`
      const wanted = await api.wanted()
      missing = wanted.find((w) => key(w.title) === key(t)) ?? { title: t, from: [] }
    }
  }

  function scrollToHash() {
    if (!location.hash) return
    document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView()
  }

  // Wiki-style section numbers: 1, 1.1, 1.2, 2 …
  const toc = $derived.by(() => {
    const heads = view?.headings.filter((h) => h.level >= 2 && h.level <= 4) ?? []
    const top = Math.min(...heads.map((h) => h.level))
    const counters: number[] = []
    return heads.map((h) => {
      const depth = h.level - top
      counters.length = depth + 1
      counters[depth] = (counters[depth] ?? 0) + 1
      for (let i = 0; i < depth; i++) counters[i] ||= 1
      return { ...h, depth, number: counters.join('.') }
    })
  })

  const modified = $derived(
    view ? new Date(view.modified).toLocaleString(store.world?.language, { dateStyle: 'long', timeStyle: 'short' }) : '',
  )
</script>

{#if error}
  <p class="error">{error}</p>
{:else if missing}
  <h1 class="page-title missing">{title}</h1>
  <p>Статті з такою назвою ще немає.</p>
  {#if missing.from.length}
    <p class="muted">
      На неї посилаються:
      {#each missing.from as from, i (from)}{i ? ', ' : ''}<a href={pageHref(from)}>{from}</a>{/each}
    </p>
  {/if}
  <p><a class="button primary" href={editHref(title)}>Створити статтю</a></p>
{:else if view}
  <div class="article" class:has-toc={toc.length > 1}>
    <article>
      <header>
        <div class="title-row">
          <h1 class="page-title">{view.title}</h1>
          <a href={editHref(view.title)} class="edit">Редагувати</a>
        </div>
        {#if view.redirectedFrom}
          <p class="note">Перенаправлено з «{view.redirectedFrom}»</p>
        {/if}
      </header>

      {#if view.type === 'disambiguation'}
        <p class="hatnote">
          Це сторінка неоднозначності: перелік статей, які можуть мати назву «{view.title}».
        </p>
      {/if}

      {#if view.infobox.length || view.image}
        <aside class="infobox">
          <div class="ib-title">{view.title}</div>
          {#if view.image}<div class="ib-image">{@html view.image}</div>{/if}
          {#if view.typeLabel}<div class="ib-type">{view.typeLabel}</div>{/if}
          <dl>
            {#each view.infobox as row (row.label)}
              <dt>{row.label}</dt>
              <dd>{@html row.html}</dd>
            {/each}
          </dl>
        </aside>
      {/if}

      <div class="prose">{@html view.html}</div>

      <footer>
        {#if view.categories.length}
          <div class="categories">
            <span class="muted">Категорії:</span>
            {#each view.categories as c (c)}<a href={categoryHref(c)}>{c}</a>{/each}
          </div>
        {/if}

        {#if view.aliases.length}
          <p class="muted">Також відомо як: {view.aliases.join(', ')}</p>
        {/if}

        <details open={view.backlinks.length > 0 && view.backlinks.length <= 10}>
          <summary>Посилання сюди <span class="muted">({view.backlinks.length})</span></summary>
          {#if view.backlinks.length}
            <ul>
              {#each view.backlinks as b (b)}<li><a href={pageHref(b)}>{b}</a></li>{/each}
            </ul>
          {:else}
            <p class="muted">Жодна стаття сюди не посилається.</p>
          {/if}
        </details>

        <p class="meta muted">Змінено {modified} · <code>{view.path}</code></p>
      </footer>
    </article>

    {#if toc.length > 1}
      <nav class="toc" aria-label="Зміст">
        <div class="toc-title">Зміст</div>
        <ol>
          {#each toc as h (h.id)}
            <li style:padding-left="{h.depth * 0.9}rem">
              <a href="#{h.id}"><span class="num">{h.number}</span> {h.text}</a>
            </li>
          {/each}
        </ol>
      </nav>
    {/if}
  </div>
  {#if view.backlinks.length === 0 && !view.categories.length}
    <p class="hint muted">
      Стаття ще ні з чим не пов'язана: додайте категорії у frontmatter або посилання [[{view.title}]] з інших статей.
    </p>
  {/if}
{/if}

<style>
  .article {
    display: grid;
    grid-template-columns: minmax(0, 52rem);
    gap: 3rem;
  }
  .article.has-toc {
    grid-template-columns: minmax(0, 52rem) 14rem;
  }
  .title-row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
  }
  .edit {
    font-size: 0.9rem;
    white-space: nowrap;
  }
  .note {
    margin: 0;
    font-size: 0.9rem;
    color: var(--text-muted);
  }
  .missing {
    color: var(--link-new);
  }
  .hatnote {
    font-style: italic;
    color: var(--text-muted);
    padding-left: 1.2rem;
  }
  header {
    margin-bottom: 1rem;
    padding-bottom: 0.6rem;
    border-bottom: 1px solid var(--border);
  }

  .infobox {
    float: right;
    width: 18rem;
    margin: 0 0 1rem 1.5rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
    font-size: 0.9rem;
    overflow: hidden;
  }
  .ib-title {
    font-weight: 700;
    font-size: 1.05rem;
    text-align: center;
    padding: 0.6rem 0.8rem;
    background: var(--accent-soft);
  }
  .ib-image :global(img) {
    display: block;
    width: 100%;
  }
  .ib-type {
    text-align: center;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-muted);
    padding: 0.4rem;
    border-bottom: 1px solid var(--border);
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    margin: 0;
    padding: 0.4rem 0.8rem 0.6rem;
    gap: 0.35rem 0.8rem;
  }
  dt {
    font-weight: 600;
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  footer {
    clear: both;
    margin-top: 2.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border);
    font-size: 0.92rem;
  }
  .categories {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    align-items: center;
    margin-bottom: 0.8rem;
  }
  .categories a {
    padding: 0.05rem 0.55rem;
    border-radius: 999px;
    background: var(--accent-soft);
  }
  details summary {
    cursor: pointer;
    font-weight: 600;
  }
  details ul {
    columns: 2 12rem;
    margin: 0.4rem 0;
  }
  .meta {
    font-size: 0.82rem;
    margin-top: 1.2rem;
  }
  .meta code {
    font-family: var(--mono);
  }
  .hint {
    font-size: 0.85rem;
  }
  .error {
    color: var(--danger);
  }

  .toc {
    position: sticky;
    top: 2rem;
    align-self: start;
    max-height: calc(100vh - 4rem);
    overflow-y: auto;
    font-size: 0.88rem;
  }
  .toc-title {
    font-weight: 700;
    margin-bottom: 0.4rem;
  }
  .toc ol {
    list-style: none;
    margin: 0;
    padding: 0;
    border-left: 2px solid var(--border);
  }
  .toc a {
    display: block;
    padding: 0.15rem 0.6rem;
    color: var(--text-muted);
  }
  .toc a:hover {
    color: var(--text);
    text-decoration: none;
  }
  .num {
    color: var(--text-muted);
    opacity: 0.7;
    margin-right: 0.2rem;
  }

  @media (max-width: 1150px) {
    .article.has-toc {
      grid-template-columns: minmax(0, 52rem);
    }
    .toc {
      display: none;
    }
  }
  @media (max-width: 640px) {
    .infobox {
      float: none;
      width: 100%;
      margin: 0 0 1rem;
    }
  }
</style>
