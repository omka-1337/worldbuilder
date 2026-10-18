<script lang="ts">
  import { tick } from 'svelte'
  import Icon from './Icon.svelte'
  import { api, pageHref } from '../lib/api'
  import { navigate } from '../lib/router.svelte'
  import { lightbox, closeLightbox, fileName, type Shot } from '../lib/lightbox.svelte'

  // Two sets to step through: the article's pictures, in the order it
  // shows them, or every picture of the world. Each picture lists the
  // articles showing it, so from any of them one can go to another article.
  type Mode = 'page' | 'world'
  let mode = $state<Mode>('page')
  let world = $state<{ name: string; url: string; pages: string[] }[] | null>(null)
  let index = $state(0)
  let strip = $state<HTMLElement>()

  const shots = $derived<Shot[]>(
    mode === 'page' ? lightbox.shots : (world ?? []).map((w) => ({ url: w.url })),
  )
  const current = $derived(shots[index])
  const pagesOf = $derived(new Map((world ?? []).map((w) => [w.url, w.pages])))
  const usedIn = $derived(current ? (pagesOf.get(current.url) ?? []) : [])

  // Every opening starts on the picture clicked, among the article's own.
  $effect(() => {
    if (!lightbox.open) return
    mode = 'page'
    index = lightbox.index
    world = null
    api.images().then((list) => (world = list))
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => (document.body.style.overflow = overflow)
  })

  // The current thumbnail stays in view.
  $effect(() => {
    void index
    void mode
    tick().then(() => strip?.querySelector('.current')?.scrollIntoView({ block: 'nearest', inline: 'center' }))
  })

  function go(to: number) {
    if (shots.length) index = (to + shots.length) % shots.length
  }

  function switchMode(to: Mode) {
    if (to === mode || (to === 'world' && !world)) return
    const url = current?.url
    mode = to
    index = Math.max(0, shots.findIndex((s) => s.url === url))
  }

  function openArticle(title: string) {
    closeLightbox()
    navigate(pageHref(title))
  }

  function onKeydown(e: KeyboardEvent) {
    if (!lightbox.open) return
    const keys: Record<string, () => void> = {
      Escape: closeLightbox,
      ArrowLeft: () => go(index - 1),
      ArrowRight: () => go(index + 1),
      Home: () => go(0),
      End: () => go(shots.length - 1),
    }
    if (keys[e.key]) {
      e.preventDefault()
      keys[e.key]()
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if lightbox.open && current}
  <div class="lightbox" role="dialog" aria-modal="true" aria-label="Перегляд зображень">
    <header>
      <span class="counter">{index + 1} / {shots.length}</span>
      <div class="modes" role="group" aria-label="Які зображення гортати">
        <button class:on={mode === 'page'} onclick={() => switchMode('page')}>Ця стаття</button>
        <button class:on={mode === 'world'} onclick={() => switchMode('world')} disabled={!world}>
          Увесь світ{world ? ` · ${world.length}` : ''}
        </button>
      </div>
      <a class="tool" href={current.url} target="_blank" rel="noopener" title="Відкрити оригінал у новій вкладці">
        Оригінал
      </a>
      <button class="tool close" onclick={closeLightbox} title="Закрити (Esc)" aria-label="Закрити">
        <Icon name="close" size={22} />
      </button>
    </header>

    <!-- A click beside the picture closes, as on any lightbox. -->
    <div class="stage" role="presentation" onclick={(e) => e.target === e.currentTarget && closeLightbox()}>
      {#if shots.length > 1}
        <button class="nav prev" onclick={() => go(index - 1)} aria-label="Попереднє (←)">
          <Icon name="left" size={28} />
        </button>
      {/if}
      {#key current.url}
        <img src={current.url} alt={current.caption ?? fileName(current.url)} />
      {/key}
      {#if shots.length > 1}
        <button class="nav next" onclick={() => go(index + 1)} aria-label="Наступне (→)">
          <Icon name="right" size={28} />
        </button>
      {/if}
    </div>

    <footer>
      <div class="info">
        <span class="caption">{current.caption || fileName(current.url)}</span>
        {#if usedIn.length}
          <span class="used">
            {usedIn.length > 1 ? 'У статтях:' : 'У статті:'}
            {#each usedIn as title, i (title)}{i ? ', ' : ''}<a
                href={pageHref(title)}
                onclick={(e) => {
                  e.preventDefault()
                  openArticle(title)
                }}>{title}</a
              >{/each}
          </span>
        {:else if world}
          <span class="used">Не використане в жодній статті</span>
        {/if}
      </div>
      {#if shots.length > 1}
        <div class="strip" bind:this={strip}>
          {#each shots as s, i (s.url + i)}
            <button class="thumb" class:current={i === index} onclick={() => (index = i)} aria-label={s.caption || fileName(s.url)}>
              <img src={s.url} alt="" loading="lazy" />
            </button>
          {/each}
        </div>
      {/if}
    </footer>
  </div>
{/if}

<style>
  .lightbox {
    position: fixed;
    inset: 0;
    z-index: 300;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    grid-template-columns: minmax(0, 1fr); /* a long strip scrolls, never widens */
    background: rgb(10 10 12 / 0.97);
    backdrop-filter: blur(6px);
    color: #eee;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.6rem 1rem;
  }
  .counter {
    font-variant-numeric: tabular-nums;
    color: #aaa;
    min-width: 4rem;
  }
  .modes {
    display: flex;
    margin-right: auto;
    border: 1px solid #444;
    border-radius: 7px;
    overflow: hidden;
  }
  .modes button {
    border: none;
    border-radius: 0;
    padding: 0.3rem 0.8rem;
    background: none;
    color: #bbb;
    font-size: 0.88rem;
  }
  .modes button.on {
    background: #ffffff1f;
    color: #fff;
  }
  .modes button:hover:not(.on):not(:disabled) {
    background: #ffffff10;
  }
  .tool {
    display: grid;
    place-items: center;
    padding: 0.3rem 0.5rem;
    border: none;
    border-radius: 6px;
    background: none;
    color: #ccc;
    font-size: 0.88rem;
  }
  .tool:hover {
    background: #ffffff14;
    color: #fff;
    text-decoration: none;
  }
  .stage {
    position: relative;
    min-height: 0;
    overflow: hidden;
  }
  /* Positioned against the stage, whose size the layout fixes, so a tall
     picture shrinks to fit instead of pushing the footer away. */
  .stage img {
    position: absolute;
    inset: 0;
    margin: auto;
    max-width: calc(100% - 9rem);
    max-height: 100%;
    border-radius: 4px;
    box-shadow: 0 10px 40px rgb(0 0 0 / 0.5);
  }
  .nav {
    position: absolute;
    top: 50%;
    translate: 0 -50%;
    display: grid;
    place-items: center;
    width: 3rem;
    height: 3rem;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: #ffffff14;
    color: #eee;
  }
  .nav:hover {
    background: #ffffff2a;
  }
  .prev {
    left: 1rem;
  }
  .next {
    right: 1rem;
  }
  footer {
    padding: 0.6rem 1rem 0.8rem;
  }
  .info {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 0.3rem 1.5rem;
    margin-bottom: 0.6rem;
    font-size: 0.9rem;
    text-align: center;
  }
  .used {
    color: #aaa;
  }
  .used a {
    color: #b8c2ff;
  }
  .strip {
    display: flex;
    gap: 0.4rem;
    justify-content: safe center;
    overflow-x: auto;
    padding-bottom: 0.2rem;
  }
  .thumb {
    flex: none;
    width: 4.5rem;
    height: 3.2rem;
    padding: 0;
    border: 2px solid transparent;
    border-radius: 5px;
    background: #222;
    overflow: hidden;
    opacity: 0.6;
  }
  .thumb:hover {
    opacity: 0.9;
    background: #222;
  }
  .thumb.current {
    border-color: #fff;
    opacity: 1;
  }
  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  @media (max-width: 640px) {
    .stage img {
      max-width: calc(100% - 1rem);
    }
    .nav {
      display: none;
    }
  }
</style>
