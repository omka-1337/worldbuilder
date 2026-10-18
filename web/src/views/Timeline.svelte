<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import { api, currentWorld, pageHref, worldHref, type Timeline, type TimelineEntry, type TimelinePoint } from '../lib/api'
  import { navigate, router } from '../lib/router.svelte'
  import { store } from '../lib/store.svelte'
  import { plural } from '../lib/format'
  import { typeColor } from '../lib/colors'

  // The server reads dates with the world's calendar and places them: on one
  // shared axis, or on several (tabs) when the calendar cannot say how long
  // the units above its step are. Positions count in the step unit.
  let data = $state<Timeline | null>(null)
  let width = $state(0) // of the stage, which narrows when the panel opens
  let stage = $state<HTMLDivElement>()

  // The visible stretch of the axis. Zooming and panning change it;
  // selecting a point animates it.
  let from = $state(0)
  let to = $state(1)

  const PAD = 70 // px kept clear at both ends of the axis
  const FOCUS = 0.5 // a selected point settles in the middle of the visible stage

  const axes = $derived(data?.axes ?? [])
  const axis = $derived(router.query.get('axis') ?? axes.at(-1)?.key ?? '')
  const axisPoints = $derived(data ? data.points.filter((p) => p.axis === axis) : null)
  const boundaries = $derived(data?.boundaries ?? [])
  const now = $derived(data?.now && data.now.axis === axis ? data.now : null)

  // Filters, remembered per world in this browser: types hidden altogether,
  // and types whose bars are hidden while their dots stay.
  const filterKey = `worldbuilder:timeline:${currentWorld()}`
  const filters = $state<{ hidden: string[]; noBars: string[] }>(loadFilters())
  function loadFilters() {
    try {
      return { hidden: [], noBars: [], ...JSON.parse(localStorage.getItem(filterKey) ?? '{}') }
    } catch {
      return { hidden: [], noBars: [] }
    }
  }
  $effect(() => {
    try {
      localStorage.setItem(filterKey, JSON.stringify(filters))
    } catch {
      // Storage unavailable: filters last for this visit.
    }
  })
  const toggle = (list: string[], type: string) => {
    const i = list.indexOf(type)
    if (i >= 0) list.splice(i, 1)
    else list.push(type)
  }

  // The points shown: hidden types drop out, and a point's name goes to its
  // most important entry still visible.
  const points = $derived.by(() => {
    if (!axisPoints) return null
    const out: TimelinePoint[] = []
    for (const p of axisPoints) {
      const entries = p.entries.filter((e) => !filters.hidden.includes(e.type ?? ''))
      if (entries.length) out.push({ ...p, entries, main: entries[0] })
    }
    return out
  })

  const keyOf = (p: { axis: string; pos: number }) => `${p.axis}|${p.pos}`

  // Where an entry's bar ends: its own end, the present for an open period,
  // or off the end of the axis when that lies on another one.
  function endPos(e: TimelineEntry) {
    if (e.hasEnd) return e.endAxis === e.axis ? e.endPos! : Infinity
    if (e.openEnd) return now ? now.pos : Infinity
    return e.pos
  }

  const name = (e: TimelineEntry) => (e.event ? e.title : `${e.title} (${e.field.toLowerCase()})`)
  const fullDate = (e: TimelineEntry) => (e.endField ? `${e.date} — ${e.endDate || 'досі'}` : e.date)

  const selectedKey = $derived(router.query.has('at') ? `${axis}|${router.query.get('at')}` : null)
  const selected = $derived(points?.find((p) => keyOf(p) === selectedKey) ?? null)
  const selectedIndex = $derived(points && selected ? points.indexOf(selected) : -1)

  document.title = `Таймлайн — ${store.world?.name}`

  onMount(async () => {
    data = await api.timeline()
  })

  // A fresh view for each axis shown.
  let shownAxis: string | null = null
  $effect(() => {
    if (!points || axis === shownAxis) return
    shownAxis = axis
    untrack(() => ([from, to] = fullRange()))
  })

  function fullRange(): [number, number] {
    if (!points?.length) return [0, 1]
    const first = Math.min(...points.map((p) => p.pos))
    // Ranges can end after the last point that starts anything.
    const ends = points.flatMap((p) => [p.pos, ...p.entries.map(endPos).filter(Number.isFinite)])
    const last = Math.max(...ends)
    const margin = Math.max((last - first) * 0.04, 5)
    return [first - margin, last + margin]
  }

  // A span of the given width that puts position p at the FOCUS point.
  function focusRange(p: number, span: number): [number, number] {
    const inner = Math.max(width - 2 * PAD, 1)
    const left = ((FOCUS * width - PAD) / inner) * span
    return [p - left, p - left + span]
  }

  const x = (p: number) => PAD + ((p - from) / (to - from)) * Math.max(width - 2 * PAD, 1)

  // More events in a step, bigger dot.
  const radius = (p: TimelinePoint) => Math.min(28, 6 + 5 * Math.sqrt(p.entries.length - 1))

  const labelHalf = (p: TimelinePoint) => Math.min(170, Math.max(name(p.main).length, 8) * 7.4) / 2 + 8

  // Shifts a label sideways so it stays inside the stage near either edge.
  const labelShift = (p: TimelinePoint) => {
    const cx = x(p.pos)
    const half = labelHalf(p)
    return Math.max(half - cx, 0) - Math.max(cx + half - width, 0)
  }

  // Labels compete for space: the selected point first, then the busiest,
  // above the axis, then below; a label that fits nowhere shows only on
  // hover.
  const placement = $derived.by(() => {
    const rows: [number, number][][] = [[], []]
    const out = new Map<string, 0 | 1>()
    if (!points) return out
    const byWeight = [...points].sort(
      (a, b) =>
        Number(b === selected) - Number(a === selected) ||
        b.entries.length - a.entries.length ||
        b.main.text - a.main.text,
    )
    for (const p of byWeight) {
      const dot = x(p.pos)
      if (dot < 0 || dot > width) continue // off-screen dots get no label
      const cx = dot + labelShift(p)
      const half = labelHalf(p)
      for (const row of [0, 1] as const) {
        if (rows[row].every(([a, b]) => cx + half < a || cx - half > b)) {
          rows[row].push([cx - half, cx + half])
          out.set(keyOf(p), row)
          break
        }
      }
    }
    return out
  })

  // Ranges and periods are drawn as bars below the axis. Bars that overlap
  // go to separate lanes; a lane is reused once its last bar has ended.
  const SPANS_TOP = 110 // px below the axis, clear of the lower labels
  const LANE = 26 // px per lane

  const spans = $derived.by(() => {
    if (!points) return []
    const ranges = points
      .flatMap((p) => p.entries.filter((e) => e.bar && !filters.noBars.includes(e.type ?? '')))
      .map((e) => ({ ...e, start: e.pos, end: endPos(e) }))
    ranges.sort((a, b) => a.start - b.start || b.end - a.end)
    const laneEnds: number[] = []
    return ranges.map((r) => {
      let lane = laneEnds.findIndex((end) => end < r.start)
      if (lane < 0) lane = laneEnds.push(0) - 1
      laneEnds[lane] = r.end
      return { ...r, lane }
    })
  })

  // Ranges still going on at the selected point, started earlier.
  const ongoing = $derived.by(() => {
    if (!selected) return []
    return spans.filter((s) => s.start < selected.pos && s.end >= selected.pos)
  })

  // Every type on this axis, hidden ones included so they can come back.
  const legend = $derived.by(() => {
    const seen = new Map<string, { label: string; bars: boolean }>()
    for (const p of axisPoints ?? [])
      for (const e of p.entries) {
        if (!e.type) continue
        const t = seen.get(e.type) ?? { label: e.typeLabel ?? e.type, bars: false }
        t.bars ||= !!e.bar
        seen.set(e.type, t)
      }
    return [...seen].map(([type, t]) => ({ type, ...t, color: typeColor(type) }))
  })
  const barTypes = $derived(legend.filter((l) => l.bars))
  let barsMenu = $state(false)

  // Axis ticks at a round step: 1, 2 or 5 × 10ⁿ, about 110px apart. On a
  // shared axis a tick is labelled by its count inside the part it falls in.
  const ticks = $derived.by(() => {
    const span = to - from
    const rough = span / Math.max((width - 2 * PAD) / 110, 1)
    const pow = 10 ** Math.floor(Math.log10(Math.max(rough, 1)))
    const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= rough) ?? 10 * pow
    const out: { at: number; label: number }[] = []
    for (let t = Math.ceil(from / step) * step; t <= to; t += step) {
      const b = [...boundaries].reverse().find((m) => m.pos <= t)
      out.push({ at: t, label: Math.round(t - (b?.pos ?? 0)) })
    }
    return out
  })
  const visibleBoundaries = $derived(boundaries.filter((b) => b.pos > from && b.pos < to))

  // --- selection --------------------------------------------------------

  function select(p: { axis: string; pos: number } | null, onAxis = axis) {
    const q = new URLSearchParams()
    if (axes.length > 1) q.set('axis', p ? p.axis : onAxis)
    if (p) q.set('at', String(p.pos))
    navigate(worldHref(undefined, '/timeline' + (q.size ? '?' + q : '')), { replace: true })
  }

  // Zoom in on a newly selected point (also when opened from a link).
  let lastFocused: string | null = null
  $effect(() => {
    const p = selected
    if (!p) {
      lastFocused = null
      return
    }
    if (keyOf(p) === lastFocused || !width) return
    lastFocused = keyOf(p)
    untrack(() => {
      const [min, max] = fullRange()
      const span = Math.min(to - from, Math.max(12, (max - min) / 6))
      animateTo(() => focusRange(p.pos, span))
    })
  })

  // The target is re-read every frame: the stage narrows while the panel
  // slides in, and the selected point should land at FOCUS of the final width.
  let frame = 0
  function animateTo(target: () => [number, number]) {
    cancelAnimationFrame(frame)
    const [f0, t0] = [from, to]
    const start = performance.now()
    const duration = matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 600
    const tick = (t: number) => {
      const k = duration ? Math.min((t - start) / duration, 1) : 1
      const e = 1 - (1 - k) ** 3
      const [f, tt] = target()
      from = f0 + (f - f0) * e
      to = t0 + (tt - t0) * e
      if (k < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
  }

  function onKeydown(e: KeyboardEvent) {
    if (!points?.length || (e.target as Element).closest('input, textarea')) return
    if (e.key === 'Escape' && barsMenu) barsMenu = false
    else if (e.key === 'Escape') select(null)
    else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      const i = selectedIndex < 0 ? 0 : selectedIndex + (e.key === 'ArrowRight' ? 1 : -1)
      if (points[i]) select(points[i])
    }
  }

  // --- zoom and pan -------------------------------------------------------

  $effect(() => {
    if (!stage) return
    // Sideways scrolling (two fingers on a touchpad, or Shift + wheel) pans;
    // vertical scrolling and pinching (a wheel event with ctrlKey) zoom
    // around the pointer.
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      cancelAnimationFrame(frame)
      const unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? width : 1 // lines or pages → px
      let dx = e.deltaX * unit
      let dy = e.deltaY * unit
      if (e.shiftKey && !dx) [dx, dy] = [dy, 0]
      const inner = Math.max(width - 2 * PAD, 1)
      const span = to - from

      if (Math.abs(dx) > Math.abs(dy)) {
        const shift = (dx / inner) * span
        from += shift
        to += shift
        return
      }
      const rect = stage!.getBoundingClientRect()
      const anchor = from + ((e.clientX - rect.left - PAD) / inner) * span
      const [min, max] = fullRange()
      const speed = e.ctrlKey ? 0.01 : 0.0015 // pinch deltas are small
      const next = Math.min(Math.max(span * Math.exp(dy * speed), 3), (max - min) * 3)
      const k = next / span
      from = anchor - (anchor - from) * k
      to = anchor + (to - anchor) * k
    }
    stage.addEventListener('wheel', onWheel, { passive: false })
    return () => stage!.removeEventListener('wheel', onWheel)
  })

  let drag = $state<{ x: number; from: number; to: number; moved: boolean } | null>(null)

  function onPointerDown(e: PointerEvent) {
    if (e.button !== 0 || (e.target as Element).closest('.point, .bar, .span, .filters')) return
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

<svelte:window
  onkeydown={onKeydown}
  onclick={(e) => barsMenu && !(e.target as Element).closest('.bars-filter') && (barsMenu = false)}
/>

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
      {#if axes.length > 1}
        <div class="epochs" role="tablist" aria-label="Частини шкали">
          {#each axes as a (a.key)}
            <button role="tab" class:on={a.key === axis} aria-selected={a.key === axis} onclick={() => select(null, a.key)}>
              {a.label || 'Шкала'}
            </button>
          {/each}
        </div>
      {/if}
      <span class="muted hint">Колесо або щипок — масштаб, прокрутка вбік чи перетягування — зсув, ← → — сусідні роки</span>
      <button onclick={resetView}>Весь таймлайн</button>
    </div>
    {#if legend.length}
      <div class="filters">
        <ul class="legend" aria-label="Типи на таймлайні">
          {#each legend as l (l.type)}
            <li>
              <button
                class:off={filters.hidden.includes(l.type)}
                onclick={() => toggle(filters.hidden, l.type)}
                aria-pressed={!filters.hidden.includes(l.type)}
                title={filters.hidden.includes(l.type) ? 'Показати' : 'Сховати'}
              >
                <span class="swatch" style:background={l.color}></span>{l.label}
              </button>
            </li>
          {/each}
        </ul>
        {#if barTypes.length}
          <div class="bars-filter">
            <button class="bars-toggle" onclick={() => (barsMenu = !barsMenu)} aria-expanded={barsMenu}>
              Смуги{filters.noBars.length ? ` (сховано: ${barTypes.filter((t) => filters.noBars.includes(t.type)).length})` : ''} ▾
            </button>
            {#if barsMenu}
              <div class="bars-menu" role="group" aria-label="Які смуги показувати">
                {#each barTypes as t (t.type)}
                  <label>
                    <input type="checkbox" checked={!filters.noBars.includes(t.type)} onchange={() => toggle(filters.noBars, t.type)} />
                    <span class="swatch" style:background={t.color}></span>{t.label}
                  </label>
                {/each}
                <div class="bars-all">
                  <button onclick={() => (filters.noBars = [])}>Усі</button>
                  <button onclick={() => (filters.noBars = barTypes.map((t) => t.type))}>Жодної</button>
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/if}

    {#if data && !data.points.length}
      <div class="empty">
        <p>На таймлайні поки нічого немає.</p>
        <p class="muted">
          Відкрийте <a href={worldHref(undefined, '/types')}>«Типи статей»</a>, додайте типу поле з видом «Дата» і
          заповніть його в статтях: <code>343</code> для події або <code>340–347</code> для відрізка.
        </p>
      </div>
    {:else if points}
      <div class="axis"></div>
      {#each ticks as t (t.at)}
        <div class="tick" style:left="{x(t.at)}px"><span>{t.label}</span></div>
      {/each}
      {#each visibleBoundaries as b (b.pos + b.label)}
        <div class="boundary" style:left="{x(b.pos)}px"><span>{b.label}</span></div>
      {/each}
      {#if now && now.pos >= from && now.pos <= to}
        <div class="now" style:left="{x(now.pos)}px" title="Найпізніша дата у світі">
          <span>Зараз · {now.label}</span>
        </div>
      {/if}

      {#each points as y (keyOf(y))}
        {@const r = radius(y)}
        {@const row = placement.get(keyOf(y))}
        <button
          class="point"
          class:selected={selected === y}
          class:labelled={row !== undefined}
          style:left="{x(y.pos)}px"
          style:--r="{r}px"
          style:--c={typeColor(y.main.type)}
          onclick={() => select(y)}
          aria-label="{y.label}: {name(y.main)}, {plural(y.entries.length, ['подія', 'події', 'подій'])}"
        >
          <span class="dot"></span>
          <span class="label" class:below={row === 1} style:--shift="{labelShift(y)}px">
            <span class="year">{y.label}</span>
            <span class="name">{name(y.main)}</span>
          </span>
        </button>
      {/each}

      {#each spans as s (s.title + s.field)}
        {@const x1 = x(s.start)}
        {@const x2 = s.end === Infinity ? width + 40 : x(s.end)}
        {#if x2 > -20 && x1 < width + 20}
          <button
            class="span"
            class:active={selected && Math.floor(s.start) <= selected.pos && s.end >= selected.pos}
            style:left="{x1}px"
            style:width="{Math.max(x2 - x1, 6)}px"
            style:top="calc(var(--axis) + {SPANS_TOP + s.lane * LANE}px)"
            style:--c={typeColor(s.type)}
            style:--inset="{Math.max(0, -x1)}px"
            onclick={() => select(s)}
            title="{s.title} · {fullDate(s)}"
          >
            <span>{s.title}</span>
          </button>
        {/if}
      {/each}
    {/if}
  </div>

  <aside class="panel" aria-live="polite">
    {#if selected}
      <header>
        <span class="big-dot" style:--r="{radius(selected)}px" style:--c={typeColor(selected.main.type)}></span>
        <div class="heading">
          <div class="year">{selected.label}</div>
          <a class="main" href={pageHref(selected.main.title)}>{name(selected.main)}</a>
        </div>
        <button class="icon" onclick={() => select(null)} aria-label="Закрити"><Icon name="close" size={18} /></button>
      </header>

      <div class="nav">
        <button disabled={selectedIndex <= 0} onclick={() => select(points![selectedIndex - 1])}>
          ← {points![selectedIndex - 1]?.label ?? ''}
        </button>
        <span class="muted">{plural(selected.entries.length, ['стаття', 'статті', 'статей'])}</span>
        <button disabled={selectedIndex >= points!.length - 1} onclick={() => select(points![selectedIndex + 1])}>
          {points![selectedIndex + 1]?.label ?? ''} →
        </button>
      </div>

      <ul>
        {#each selected.entries as e (e.title + e.field)}
          <li class:event={e.event} style:--c={typeColor(e.type)}>
            <a href={pageHref(e.title)}>{e.title}</a>
            <span class="meta muted">
              {e.typeLabel}{e.event ? '' : ' · ' + (e.endField ? `${e.field.toLowerCase()} — ${e.endField.toLowerCase()}` : e.field.toLowerCase())} · {fullDate(e)}
            </span>
          </li>
        {/each}
      </ul>

      {#if ongoing.length}
        <h3 class="section">Триває в цей час</h3>
        <ul>
          {#each ongoing as e (e.title + e.field)}
            <li style:--c={typeColor(e.type)}>
              <a href={pageHref(e.title)}>{e.title}</a>
              <span class="meta muted">{e.typeLabel} · {fullDate(e)}</span>
            </li>
          {/each}
        </ul>
      {/if}
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
    --axis: 40%;
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
  .epochs {
    display: inline-flex;
    border: 1px solid var(--border);
    border-radius: 7px;
    overflow: hidden;
    align-self: center;
  }
  .epochs button {
    border: none;
    border-radius: 0;
    padding: 0.2rem 0.7rem;
    font-size: 0.85rem;
    background: var(--bg-raised);
  }
  .epochs button + button {
    border-left: 1px solid var(--border);
  }
  .epochs button.on {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .hint {
    flex: 1;
    min-width: 0;
    white-space: nowrap; /* one line, or it runs into the legend below */
    overflow: hidden;
    text-overflow: ellipsis;
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

  /* Type toggles (the legend) and the bars menu, under the title bar. */
  .filters {
    position: absolute;
    top: 4rem;
    left: 1.5rem;
    right: 1.5rem;
    z-index: 4;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem 1rem;
    cursor: default;
    font-size: 0.82rem;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 0.3rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .legend button {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.1rem 0.5rem;
    border: 1px solid transparent;
    border-radius: 999px;
    background: none;
    color: var(--text-muted);
    font-size: inherit;
  }
  .legend button:hover {
    border-color: var(--border);
  }
  .legend button.off {
    opacity: 0.45;
    text-decoration: line-through;
  }
  .swatch {
    width: 0.65rem;
    height: 0.65rem;
    border-radius: 50%;
    flex: none;
  }
  .bars-filter {
    position: relative;
  }
  .bars-toggle {
    padding: 0.1rem 0.6rem;
    font-size: inherit;
  }
  .bars-menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    min-width: 13rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.14);
  }
  .bars-menu label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.2rem 0.3rem;
    border-radius: 5px;
    cursor: pointer;
  }
  .bars-menu label:hover {
    background: var(--bg-hover);
  }
  .bars-all {
    display: flex;
    gap: 0.3rem;
    margin-top: 0.3rem;
    padding-top: 0.4rem;
    border-top: 1px solid var(--border);
  }
  .bars-all button {
    padding: 0.1rem 0.5rem;
    font-size: inherit;
  }

  .axis {
    position: absolute;
    top: var(--axis);
    left: 0;
    right: 0;
    border-top: 2px solid var(--border);
  }
  .tick {
    position: absolute;
    top: var(--axis);
    height: 10px;
    border-left: 1px solid var(--border);
    pointer-events: none;
  }
  /* A faint guide down through the range bars, to read their years. */
  .tick::after {
    content: '';
    position: absolute;
    top: 34px;
    left: -1px;
    height: 100vh;
    border-left: 1px dashed color-mix(in srgb, var(--border) 60%, transparent);
  }
  /* Where an epoch begins, on an axis shared by several. */
  .boundary {
    position: absolute;
    top: 7rem;
    bottom: 0;
    border-left: 2px solid var(--text-muted);
    opacity: 0.5;
    pointer-events: none;
  }
  .boundary span {
    position: absolute;
    top: 0;
    left: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    white-space: nowrap;
  }
  /* The present: the latest date in the world. */
  .now {
    z-index: 0;
    position: absolute;
    top: 7rem;
    bottom: 0;
    border-left: 2px dashed var(--accent);
    pointer-events: none;
  }
  .now span {
    position: absolute;
    top: 0;
    left: 6px;
    padding: 0 0.3rem;
    border-radius: 4px;
    background: var(--accent-soft);
    color: var(--accent);
    font-size: 0.75rem;
    font-weight: 600;
    white-space: nowrap;
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
    top: var(--axis);
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
    background: var(--c);
    border: 3px solid var(--bg);
    box-shadow: 0 0 0 1px var(--c);
    transition: transform 0.15s ease, box-shadow 0.15s ease;
  }
  .point:hover .dot,
  .point:focus-visible .dot {
    transform: scale(1.45);
  }
  .point.selected .dot {
    transform: scale(1.6);
    box-shadow: 0 0 0 1px var(--c), 0 0 0 7px color-mix(in srgb, var(--c) 22%, transparent);
  }

  .span {
    position: absolute;
    height: 20px;
    padding: 0;
    border: none;
    border-left: 3px solid var(--c);
    border-radius: 4px;
    background: color-mix(in srgb, var(--c) 22%, var(--bg));
    color: var(--text);
    font-size: 0.78rem;
    line-height: 20px;
    text-align: left;
    overflow: hidden;
    transition: background 0.15s;
  }
  .span span {
    display: block;
    padding: 0 0.4rem 0 calc(0.4rem + var(--inset));
    white-space: nowrap;
  }
  .span:hover,
  .span.active {
    background: color-mix(in srgb, var(--c) 40%, var(--bg));
  }
  .span.active {
    font-weight: 600;
  }

  .label {
    position: absolute;
    left: 0;
    bottom: calc(var(--r) + 12px);
    transform: translateX(calc(-50% + var(--shift, 0px)));
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
  /* Lines crossing the axis (epoch borders, the present) pass under labels. */
  .labelled .label {
    background: color-mix(in srgb, var(--bg) 85%, transparent);
    border-radius: 6px;
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
    background: var(--c);
    box-shadow: 0 0 0 6px color-mix(in srgb, var(--c) 22%, transparent);
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
  .panel ul {
    list-style: none;
    margin: 0;
    padding: 0 0.75rem 0.5rem;
  }
  .panel li {
    display: flex;
    flex-direction: column;
    padding: 0.55rem 0.5rem 0.55rem 0.8rem;
    border-left: 3px solid var(--c);
    margin: 0.3rem 0;
  }
  .section {
    margin: 0.8rem 1.25rem 0.2rem;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }

  .panel li a {
    font-weight: 600;
  }
  .panel li:not(.event) a {
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
