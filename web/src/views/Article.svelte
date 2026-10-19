<script lang="ts">
  import { tick } from 'svelte'
  import { api, key, NotFoundError, pageHref, editHref, categoryHref, type PageView, type Wanted } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { openLightbox, type Shot } from '../lib/lightbox.svelte'

  let { title }: { title: string } = $props()

  let view = $state<PageView | null>(null)
  let missing = $state<Wanted | null>(null)
  let error = $state('')

  $effect(() => {
    void store.version // reload after moves and renames elsewhere
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

  // A click on any picture (the banner, the infobox's, one in the text)
  // opens the viewer on it, with the article's pictures to step through in
  // the order they appear.
  let root = $state<HTMLElement>()
  function onPictureClick(e: MouseEvent) {
    const target = e.target as HTMLElement
    const picture = target.closest<HTMLElement>('.banner, .ib-image img, .prose img')
    if (!picture || !root || target.closest('a')) return
    const shots: Shot[] = []
    let index = 0
    for (const el of root.querySelectorAll<HTMLElement>('.banner, .ib-image img, .prose img')) {
      const url = el instanceof HTMLImageElement ? el.getAttribute('src') : view?.banner
      if (!url) continue
      if (el === picture) index = shots.length
      const caption = el.closest('.ib-image') ? root.querySelector('.ib-image figcaption')?.textContent?.trim() : el.getAttribute('alt')
      shots.push({ url, caption: caption || undefined })
    }
    openLightbox(shots, index)
  }

  // --- chronology: the text's {{date | description}} anchors --------------

  let tab = $state<'article' | 'chronology'>('article')

  // From an entry back to its place in the text, briefly lit up.
  async function goTo(id: string) {
    tab = 'article'
    await tick()
    const el = document.getElementById(id)
    if (!el) return
    el.scrollIntoView({ block: 'center', behavior: 'smooth' })
    el.classList.remove('flash')
    void el.offsetWidth // restart the animation
    el.classList.add('flash')
  }

  function scrollToHash() {
    if (!location.hash) return
    document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView()
  }

  // Wiki-style section numbers: 1, 1.1, 1.2, 2 …
  const toc = $derived.by(() => {
    // Sections usually start at ## (the title is the page's own heading),
    // but notes from other tools often use # for them: start at whatever the
    // article's top level is and go three levels deep.
    const all = view?.headings ?? []
    const top = Math.min(...all.map((h) => h.level))
    const heads = all.filter((h) => h.level <= top + 2)
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
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div class="pictures" bind:this={root} onclick={onPictureClick}>
  {#if view.banner}
    <div class="banner" style:background-image="url('{view.banner}')" style:background-position-y="{(view.bannerY ?? 0.5) * 100}%"></div>
  {/if}
  <div class="article" class:has-toc={toc.length > 1}>
    <article>
      <header>
        {#if view.location.length}
          <nav class="crumbs" aria-label="Розташування">
            {#each view.location as c, i (i)}
              {#if c.title}<a href={pageHref(c.title)}>{c.name}</a>{:else}<span>{c.name}</span>{/if}
              <span class="sep" aria-hidden="true">›</span>
            {/each}
            <span class="here" aria-current="page">{view.title}</span>
          </nav>
        {/if}
        <div class="title-row">
          <h1 class="page-title">{view.title}</h1>
          <a href={editHref(view.title)} class="edit">Редагувати</a>
        </div>
        {#if view.redirectedFrom}
          <p class="note">Перенаправлено з «{view.redirectedFrom}»</p>
        {/if}
        {#if view.chronology}
          <div class="tabs" role="tablist" aria-label="Подання статті">
            <button role="tab" aria-selected={tab === 'article'} class:on={tab === 'article'} onclick={() => (tab = 'article')}>
              Стаття
            </button>
            <button role="tab" aria-selected={tab === 'chronology'} class:on={tab === 'chronology'} onclick={() => (tab = 'chronology')}>
              Хронологія <span class="count">{view.chronology.length}</span>
            </button>
          </div>
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
          {#if view.image}
            <figure class="ib-image">
              {@html view.image}
              {#if view.caption}<figcaption>{@html view.caption}</figcaption>{/if}
            </figure>
          {/if}
          {#if view.typeLabel}<div class="ib-type">{view.typeLabel}</div>{/if}
          {#if view.infobox.length}<dl>
            {#each view.infobox as row, i (i)}
              {#if row.section}
                <dt class="ib-section">{row.label}</dt>
              {:else}
                <dt>{row.label}</dt>
                <dd>{@html row.html}</dd>
              {/if}
            {/each}
          </dl>{/if}
        </aside>
      {/if}

      <div class="prose" hidden={tab !== 'article'}>{@html view.html}</div>

      {#if tab === 'chronology' && view.chronology}
        <section class="chronology" aria-label="Хронологія">
            <ol>
              {#each view.chronology as e (e.id)}
                <li>
                  <button class="entry" onclick={() => goTo(e.id)} title="Показати в тексті">
                    <span class="when">{e.date}</span>
                    {#if e.note}<span class="what">{e.note}</span>{/if}
                  </button>
                </li>
              {/each}
            </ol>
        </section>
      {/if}

      <footer>
        {#if view.categories.length}
          <div class="categories">
            <span class="muted">Категорії:</span>
            {#each view.categories as c (c)}<a href={categoryHref(c)}>{c}</a>{/each}
          </div>
        {/if}

        {#if view.children.length}
          <div class="children">
            <h2>У цьому розділі</h2>
            <ul>
              {#each view.children as c (c)}<li><a href={pageHref(c)}>{c}</a></li>{/each}
            </ul>
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

    {#if toc.length > 1 && tab === 'article'}
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
  </div>
  {#if view.backlinks.length === 0 && !view.categories.length}
    <p class="hint muted">
      Стаття ще ні з чим не пов'язана: додайте їй категорії в редакторі або послайтеся на неї з інших статей.
    </p>
  {/if}
{/if}

<style>
  /* The article takes all the room there is; the contents keep a column. */
  .article {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 3rem;
  }
  .article.has-toc {
    grid-template-columns: minmax(0, 1fr) 14rem;
  }
  .crumbs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    font-size: 0.85rem;
    color: var(--text-muted);
    margin-bottom: 0.2rem;
  }
  .crumbs .sep {
    opacity: 0.6;
  }
  .crumbs .here {
    color: var(--text);
  }
  .children {
    margin-bottom: 1rem;
  }
  .children h2 {
    font-size: 0.95rem;
    margin: 0 0 0.3rem;
  }
  .children ul {
    columns: 2 12rem;
    margin: 0;
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
  .ib-image {
    margin: 0;
  }
  .ib-image :global(img) {
    display: block;
    width: 100%;
    max-height: 22rem; /* a tall portrait must not stretch the infobox */
    object-fit: contain;
    background: var(--bg-sidebar);
  }
  figcaption {
    padding: 0.4rem 0.8rem;
    font-size: 0.82rem;
    color: var(--text-muted);
    text-align: center;
    border-bottom: 1px solid var(--border);
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
  .ib-section {
    grid-column: 1 / -1;
    margin: 0.4rem -0.8rem 0;
    padding: 0.3rem 0.8rem;
    background: var(--accent-soft);
    text-align: center;
    font-size: 0.85rem;
  }
  .ib-section:first-child {
    margin-top: -0.4rem;
  }
  .banner,
  .ib-image :global(img),
  .prose :global(img) {
    cursor: zoom-in;
  }
  .tabs {
    display: flex;
    gap: 0.2rem;
    margin: 0.6rem 0 -0.6rem;
  }
  .tabs button {
    padding: 0.35rem 0.8rem;
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    background: none;
    color: var(--text-muted);
    font-size: 0.92rem;
  }
  .tabs button:hover {
    color: var(--text);
    background: none;
  }
  .tabs button.on {
    color: var(--text);
    border-bottom-color: var(--accent);
    font-weight: 600;
  }
  .tabs .count {
    font-weight: 400;
    color: var(--text-muted);
  }
  .chronology {
    overflow: hidden; /* sits beside the floated infobox, as one column */
    padding-left: 6px; /* room for the dots on the line */
  }
  .chronology ol {
    list-style: none;
    margin: 0;
    padding: 0;
    border-left: 2px solid var(--border);
  }
  .chronology li {
    position: relative;
    padding-left: 1rem;
  }
  .chronology li::before {
    content: '';
    position: absolute;
    left: -6px;
    top: 0.95rem;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--bg);
    border: 2px solid var(--accent);
  }
  .entry {
    width: 100%;
    display: grid;
    gap: 0.1rem;
    padding: 0.45rem 0.6rem;
    border: none;
    border-radius: 6px;
    background: none;
    text-align: left;
    font: inherit;
    color: var(--text);
  }
  .entry:hover {
    background: var(--bg-hover);
  }
  .when {
    font-weight: 650;
    color: var(--accent);
  }
  .what {
    line-height: 1.5;
  }
  .banner {
    height: clamp(9rem, 22vh, 15rem);
    margin-bottom: 1.25rem;
    border-radius: 10px;
    background-size: cover;
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
      grid-template-columns: minmax(0, 1fr);
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
