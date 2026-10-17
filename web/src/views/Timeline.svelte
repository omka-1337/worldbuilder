<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import { api, pageHref, worldHref, type TimelineEntry, type TimelineYear } from '../lib/api'
  import { navigate, router } from '../lib/router.svelte'
  import { store } from '../lib/store.svelte'
  import { plural } from '../lib/format'

  let years = $state<TimelineYear[] | null>(null)
  let width = $state(0) // of the stage, which narrows when the panel opens
  let stage = $state<HTMLDivElement>()

  // The visible span of years. Zooming and panning change it; selecting a
  // year animates it.
  let from = $state(0)
  let to = $state(1)

  const PAD = 70 // px kept clear at both ends of the axis
  const FOCUS = 0.78 // where a selected year settles, as a fraction of the stage

  const era = $derived(store.world?.era ? ' ' + store.world.era : '')
  const selectedYear = $derived(router.query.has('year') ? Number(router.query.get('year')) : null)
  const selected = $derived(years?.find((y) => y.year === selectedYear) ?? null)
  const selectedIndex = $derived(years && selected ? years.indexOf(selected) : -1)

  document.title = `Таймлайн — ${store.world?.name}`

  onMount(async () => {
    years = await api.timeline()
    ;[from, to] = fullRange()
  })

  function fullRange(): [number, number] {
    if (!years?.length) return [0, 1]
    const first = years[0].year
    const last = years[years.length - 1].year
    const margin = Math.max((last - first) * 0.04, 5)
    return [first - margin, last + margin]
  }

  // A span of the given width that puts `year` at the FOCUS point.
  function focusRange(year: number, span: number): [number, number] {
    const inner = Math.max(width - 2 * PAD, 1)
    const left = ((FOCUS * width - PAD) / inner) * span
    return [year - left, year - left + span]
  }

  const x = (year: number) => PAD + ((year - from) / (to - from)) * Math.max(width - 2 * PAD, 1)

  // More events in a year, bigger dot.
  const radius = (y: TimelineYear) => Math.min(28, 6 + 5 * Math.sqrt(y.entries.length - 1))

  const name = (e: TimelineEntry) => (e.event ? e.title : `${e.title} (${e.field.toLowerCase()})`)
  const range = (e: TimelineEntry) => `${e.year}${e.endYear ? '–' + e.endYear : ''}${era}`

  // Labels compete for space: the busiest years claim a spot first, above
  // the axis, then below; a label that fits nowhere shows only on hover.
  const placement = $derived.by(() => {
    const rows: [number, number][][] = [[], []]
    const out = new Map<number, 0 | 1>()
    if (!years) return out
    const byWeight = [...years].sort((a, b) => b.entries.length - a.entries.length || b.main.text - a.main.text)
    for (const y of byWeight) {
      const cx = x(y.year)
      if (cx < -100 || cx > width + 100) continue
      const half = Math.min(170, Math.max(name(y.main).length, 8) * 7.4) / 2 + 8
      for (const row of [0, 1] as const) {
        if (rows[row].every(([a, b]) => cx + half < a || cx - half > b)) {
          rows[row].push([cx - half, cx + half])
          out.set(y.year, row)
          break
        }
      }
    }
    return out
  })

  // Axis ticks at a round step: 1, 2 or 5 × 10ⁿ years, about 110px apart.
  const ticks = $derived.by(() => {
    const span = to - from
    const rough = span / Math.max((width - 2 * PAD) / 110, 1)
    const pow = 10 ** Math.floor(Math.log10(Math.max(rough, 1)))
    const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= rough) ?? 10 * pow
    const out: number[] = []
    for (let t = Math.ceil(from / step) * step; t <= to; t += step) out.push(t)
    return out
  })

  // --- selection --------------------------------------------------------

  function select(year: number | null) {
    const href = worldHref(undefined, '/timeline' + (year === null ? '' : `?year=${year}`))
    navigate(href, { replace: true })
  }

  // Zoom in on a newly selected year (also when opened from a ?year= link).
  let lastFocused: number | null = null
  $effect(() => {
    const y = selected
    if (!y) {
      lastFocused = null
      return
    }
    if (y.year === lastFocused || !width) return
    lastFocused = y.year
    untrack(() => {
      const [min, max] = fullRange()
      const span = Math.min(to - from, Math.max(12, (max - min) / 6))
      animateTo(() => focusRange(y.year, span))
    })
  })

  // The target is re-read every frame: the stage narrows while the panel
  // slides in, and the selected year should land at FOCUS of the final width.
  let frame = 0
  function animateTo(target: () => [number, number]) {
    cancelAnimationFrame(frame)
    const [f0, t0] = [from, to]
    const start = performance.now()
    const duration = matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 600
    const tick = (now: number) => {
      const k = duration ? Math.min((now - start) / duration, 1) : 1
      const e = 1 - (1 - k) ** 3
      const [f, t] = target()
      from = f0 + (f - f0) * e
      to = t0 + (t - t0) * e
      if (k < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
  }

  function onKeydown(e: KeyboardEvent) {
    if (!years?.length || (e.target as Element).closest('input, textarea')) return
    if (e.key === 'Escape') select(null)
    else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      const i = selectedIndex < 0 ? 0 : selectedIndex + (e.key === 'ArrowRight' ? 1 : -1)
      if (years[i]) select(years[i].year)
    }
  }

  // --- zoom and pan -------------------------------------------------------

  $effect(() => {
    if (!stage) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      cancelAnimationFrame(frame)
      const rect = stage!.getBoundingClientRect()
      const anchor = from + ((e.clientX - rect.left - PAD) / Math.max(width - 2 * PAD, 1)) * (to - from)
      const [min, max] = fullRange()
      const span = to - from
      const next = Math.min(Math.max(span * Math.exp(e.deltaY * 0.0015), 3), (max - min) * 3)
      const k = next / span
      from = anchor - (anchor - from) * k
      to = anchor + (to - anchor) * k
    }
    stage.addEventListener('wheel', onWheel, { passive: false })
    return () => stage!.removeEventListener('wheel', onWheel)
  })

  let drag = $state<{ x: number; from: number; to: number; moved: boolean } | null>(null)

  function onPointerDown(e: PointerEvent) {
    if (e.button !== 0 || (e.target as Element).closest('.point, .bar')) return
    cancelAnimationFrame(frame)
    drag = { x: e.clientX, from, to, moved: false }
    stage!.setPointerCapture(e.pointerId)
  }

  function onPointerMove(e: PointerEvent) {
    if (!drag) return
    const dx = e.clientX - drag.x
    if (Math.abs(dx) > 3) drag.moved = true
    const shift = (dx / Math.max(width - 2 * PAD, 1)) * (drag.to - drag.from)
    from = drag.from - shift
    to = drag.to - shift
  }

  function onPointerUp() {
    if (drag && !drag.moved) select(null) // a plain click on empty space closes the panel
    drag = null
  }

  function resetView() {
    select(null)
    animateTo(fullRange)
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="timeline" class:open={!!selected}>
  <div
    class="stage"
    class:dragging={drag?.moved}
    bind:this={stage}
    bind:clientWidth={width}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    role="application"
    aria-label="Таймлайн"
  >
    <div class="bar">
      <h1>Таймлайн</h1>
      <span class="muted hint">Колесо — масштаб, перетягування — зсув, ← → — сусідні роки</span>
      <button onclick={resetView}>Весь таймлайн</button>
    </div>

    {#if years && !years.length}
      <div class="empty">
        <p>На таймлайні поки нічого немає.</p>
        <p class="muted">
          Позначте поля-дати в <code>world.yaml</code> через <code>type: date</code> (наприклад, <code>date</code>
          у типу «Подія») і заповніть їх у статтях: <code>date: 343</code> або <code>date: 340–347</code>.
        </p>
      </div>
    {:else if years}
      <div class="axis"></div>
      {#each ticks as t (t)}
        <div class="tick" style:left="{x(t)}px"><span>{t}</span></div>
      {/each}

      {#each years as y (y.year)}
        {@const r = radius(y)}
        {@const row = placement.get(y.year)}
        <button
          class="point"
          class:selected={selected === y}
          class:labelled={row !== undefined || selected === y}
          style:left="{x(y.year)}px"
          style:--r="{r}px"
          onclick={() => select(y.year)}
          aria-label="{y.year}{era}: {name(y.main)}, {plural(y.entries.length, ['подія', 'події', 'подій'])}"
        >
          <span class="dot"></span>
          <span class="label" class:below={row === 1}>
            <span class="year">{y.year}{era}</span>
            <span class="name">{name(y.main)}</span>
          </span>
        </button>
      {/each}
    {/if}
  </div>

  <aside class="panel" aria-live="polite">
    {#if selected}
      <header>
        <span class="big-dot" style:--r="{radius(selected)}px"></span>
        <div class="heading">
          <div class="year">{selected.year}{era}</div>
          <a class="main" href={pageHref(selected.main.title)}>{name(selected.main)}</a>
        </div>
        <button class="icon" onclick={() => select(null)} aria-label="Закрити"><Icon name="close" size={18} /></button>
      </header>

      <div class="nav">
        <button disabled={selectedIndex <= 0} onclick={() => select(years![selectedIndex - 1].year)}>
          ← {years![selectedIndex - 1]?.year ?? ''}
        </button>
        <span class="muted">{plural(selected.entries.length, ['стаття', 'статті', 'статей'])}</span>
        <button disabled={selectedIndex >= years!.length - 1} onclick={() => select(years![selectedIndex + 1].year)}>
          {years![selectedIndex + 1]?.year ?? ''} →
        </button>
      </div>

      <ul>
        {#each selected.entries as e (e.title + e.field)}
          <li class:event={e.event}>
            <a href={pageHref(e.title)}>{e.title}</a>
            <span class="meta muted">
              {e.typeLabel}{e.event ? '' : ' · ' + e.field.toLowerCase()} · {range(e)}
            </span>
          </li>
        {/each}
      </ul>
    {/if}
  </aside>
</div>

<style>
  .timeline {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 0;
    height: 100vh;
    overflow: hidden;
    transition: grid-template-columns 0.35s ease;
  }
  .timeline.open {
    grid-template-columns: minmax(0, 1fr) min(24rem, 45vw);
  }

  .stage {
    position: relative;
    overflow: hidden;
    cursor: grab;
    user-select: none;
    touch-action: none;
  }
  .stage.dragging {
    cursor: grabbing;
  }
  .bar {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 3;
    display: flex;
    align-items: baseline;
    gap: 1rem;
    padding: 1.25rem 1.5rem;
    cursor: default;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
  }
  .hint {
    flex: 1;
    font-size: 0.82rem;
  }
  .empty {
    position: absolute;
    top: 30%;
    left: 1.5rem;
    right: 1.5rem;
    max-width: 36rem;
  }
  .empty code {
    font-family: var(--mono);
    font-size: 0.88em;
  }

  .axis {
    position: absolute;
    top: 55%;
    left: 0;
    right: 0;
    border-top: 2px solid var(--border);
  }
  .tick {
    position: absolute;
    top: 55%;
    height: 10px;
    border-left: 1px solid var(--border);
    pointer-events: none;
  }
  .tick span {
    position: absolute;
    top: 14px;
    transform: translateX(-50%);
    font-size: 0.75rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }

  /* A zero-size anchor on the axis; the dot and label hang off it. */
  .point {
    position: absolute;
    top: 55%;
    width: 0;
    height: 0;
    padding: 0;
    border: none;
    background: none;
    z-index: 1;
  }
  .point:hover,
  .point:focus-visible,
  .point.selected {
    z-index: 2;
    outline: none;
  }
  .dot {
    position: absolute;
    left: calc(var(--r) * -1);
    top: calc(var(--r) * -1);
    width: calc(var(--r) * 2);
    height: calc(var(--r) * 2);
    border-radius: 50%;
    background: var(--accent);
    border: 3px solid var(--bg);
    box-shadow: 0 0 0 1px var(--accent);
    transition: transform 0.15s ease, box-shadow 0.15s ease;
  }
  .point:hover .dot,
  .point:focus-visible .dot {
    transform: scale(1.45);
  }
  .point.selected .dot {
    transform: scale(1.6);
    box-shadow: 0 0 0 1px var(--accent), 0 0 0 7px var(--accent-soft);
  }

  .label {
    position: absolute;
    left: 0;
    bottom: calc(var(--r) + 12px);
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    width: max-content; /* the anchor is 0px wide; don't shrink to it */
    max-width: 11rem;
    text-align: center;
    line-height: 1.25;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s;
  }
  .label.below {
    bottom: auto;
    top: calc(var(--r) + 30px);
  }
  .labelled .label,
  .point:hover .label,
  .point:focus-visible .label {
    opacity: 1;
  }
  .point:hover .label {
    background: var(--bg);
    border-radius: 6px;
  }
  .label .year {
    font-size: 0.75rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .label .name {
    font-size: 0.88rem;
    font-weight: 600;
    color: var(--text);
  }
  .point.selected .label .name {
    color: var(--accent);
  }

  .panel {
    overflow-y: auto;
    border-left: 1px solid var(--border);
    background: var(--bg-raised);
    min-width: 0;
  }
  .panel header {
    display: flex;
    align-items: center;
    gap: 0.9rem;
    padding: 1.25rem 1.25rem 1rem;
    border-bottom: 1px solid var(--border);
  }
  .big-dot {
    flex: none;
    width: calc(var(--r) * 2 + 8px);
    height: calc(var(--r) * 2 + 8px);
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 0 6px var(--accent-soft);
  }
  .heading {
    flex: 1;
    min-width: 0;
  }
  .heading .year {
    font-size: 1.5rem;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    line-height: 1.2;
  }
  .heading .main {
    font-weight: 600;
  }
  .icon {
    align-self: flex-start;
    display: grid;
    padding: 0.25rem;
    border: none;
    background: none;
    color: var(--text-muted);
  }
  .nav {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.6rem 1.25rem;
    font-size: 0.85rem;
  }
  .nav button {
    padding: 0.15rem 0.6rem;
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0 0.75rem 1.5rem;
  }
  li {
    display: flex;
    flex-direction: column;
    padding: 0.55rem 0.5rem;
    border-radius: 6px;
  }
  li + li {
    border-top: 1px solid var(--border);
  }
  li a {
    font-weight: 600;
  }
  li:not(.event) a {
    font-weight: 500;
  }
  .meta {
    font-size: 0.8rem;
  }

  @media (max-width: 800px) {
    .timeline,
    .timeline.open {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(0, 1fr) auto;
      height: calc(100vh - 3.2rem);
    }
    .timeline.open .panel {
      max-height: 50vh;
      border-left: none;
      border-top: 1px solid var(--border);
    }
    .hint {
      display: none;
    }
  }
</style>
