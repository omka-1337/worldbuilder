<script lang="ts">
  import Icon, { type IconName } from './Icon.svelte'
  import { worldHref } from '../lib/api'
  import { router } from '../lib/router.svelte'
  import { prefs, type Side } from '../lib/prefs.svelte'
  import { store } from '../lib/store.svelte'
  import { changes, refreshChanges } from '../lib/versions.svelte'
  import { plural } from '../lib/format'

  const tools: { sub: string; icon: IconName; label: string }[] = [
    { sub: '/new', icon: 'plus', label: 'Нова стаття' },
    { sub: '/timeline', icon: 'timeline', label: 'Таймлайн' },
    { sub: '/pages', icon: 'list', label: 'Усі статті' },
    { sub: '/categories', icon: 'tag', label: 'Категорії' },
    { sub: '/wanted', icon: 'question', label: 'Потрібні статті' },
    { sub: '/types', icon: 'shapes', label: 'Типи статей' },
    { sub: '/calendar', icon: 'calendar', label: 'Календар' },
  ]

  const isActive = (sub: string) =>
    router.path.startsWith(new URL(worldHref(undefined, sub), location.href).pathname)

  // The history status at the bottom: refreshed after the app changes
  // something, when the window comes back into view, and every little while
  // for edits made in other programs.
  $effect(() => {
    void store.version
    refreshChanges()
  })
  $effect(() => {
    const onVisible = () => document.visibilityState === 'visible' && refreshChanges()
    const timer = setInterval(onVisible, 15_000)
    addEventListener('focus', onVisible)
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(timer)
      removeEventListener('focus', onVisible)
      document.removeEventListener('visibilitychange', onVisible)
    }
  })
  const unsaved = $derived(changes.list.length)
  const historyLabel = $derived(
    unsaved ? `Незбережені зміни: ${plural(unsaved, ['файл', 'файли', 'файлів'])}` : 'Версії: усе збережено',
  )

  // Dragging the grip picks the screen half the pointer is released over.
  let dragging = $state(false)
  let target = $state<Side>('left')

  function startDrag(e: PointerEvent) {
    if (e.button !== 0) return
    e.preventDefault()
    dragging = true
    target = prefs.ribbon
    const move = (e: PointerEvent) => (target = e.clientX < innerWidth / 2 ? 'left' : 'right')
    const up = () => {
      prefs.ribbon = target
      dragging = false
      removeEventListener('pointermove', move)
      removeEventListener('pointerup', up)
    }
    addEventListener('pointermove', move)
    addEventListener('pointerup', up)
  }

  // Keyboard users move the panel with Enter/Space on the grip.
  function onGripClick(e: MouseEvent) {
    if (e.detail === 0) prefs.ribbon = prefs.ribbon === 'left' ? 'right' : 'left'
  }
</script>

<nav class="ribbon" aria-label="Інструменти">
  <button
    class="grip"
    onpointerdown={startDrag}
    onclick={onGripClick}
    title="Перетягніть, щоб перемістити панель"
    aria-label="Перемістити панель на інший бік"
  >
    <Icon name="grip" size={16} />
  </button>
  {#each tools as tool (tool.sub)}
    <a href={worldHref(undefined, tool.sub)} class:active={isActive(tool.sub)} title={tool.label} aria-label={tool.label}>
      <Icon name={tool.icon} />
    </a>
  {/each}
  {#if changes.ready}
    <a
      href={worldHref(undefined, '/versions')}
      class="history"
      class:active={isActive('/versions')}
      title={historyLabel}
      aria-label={historyLabel}
    >
      <Icon name="history" />
      {#if unsaved}<span class="badge">{unsaved > 99 ? '99+' : unsaved}</span>{/if}
    </a>
  {/if}
</nav>

{#if dragging}
  <div class="drop-zones" aria-hidden="true">
    <div class:hot={target === 'left'}>Панель ліворуч</div>
    <div class:hot={target === 'right'}>Панель праворуч</div>
  </div>
{/if}

<style>
  .ribbon {
    position: sticky;
    top: 0;
    height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    padding: 0.5rem 0;
    background: var(--bg-sidebar);
  }
  a,
  .grip {
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    border-radius: 7px;
    color: var(--text-muted);
  }
  a:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  a.active {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .history {
    position: relative;
    margin-top: auto; /* status at the bottom of the panel */
  }
  .badge {
    position: absolute;
    top: 0;
    right: -0.15rem;
    min-width: 1.05rem;
    height: 1.05rem;
    padding: 0 0.25rem;
    border-radius: 999px;
    background: var(--accent);
    color: var(--bg);
    font-size: 0.65rem;
    font-weight: 700;
    line-height: 1.05rem;
    text-align: center;
  }
  .grip {
    border: none;
    background: none;
    padding: 0;
    height: 1.5rem;
    margin-bottom: 0.4rem;
    cursor: grab;
    touch-action: none;
  }
  .grip:hover {
    color: var(--text);
    background: none;
  }

  .drop-zones {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
    padding: 1rem;
    cursor: grabbing;
    background: rgb(0 0 0 / 0.08);
  }
  .drop-zones div {
    display: grid;
    place-items: center;
    border: 2px dashed var(--border);
    border-radius: 14px;
    color: var(--text-muted);
    font-weight: 600;
  }
  .drop-zones .hot {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent-soft) 70%, transparent);
    color: var(--accent);
  }

  @media (max-width: 800px) {
    .ribbon {
      position: fixed;
      top: auto;
      bottom: 0;
      left: 0;
      right: 0;
      z-index: 15;
      height: auto;
      flex-direction: row;
      justify-content: space-around;
      padding: 0.3rem;
      border-top: 1px solid var(--border);
    }
    .grip {
      display: none;
    }
    .history {
      margin-top: 0;
    }
  }
</style>
