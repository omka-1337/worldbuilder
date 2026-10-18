<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import ChipsInput from '../components/ChipsInput.svelte'
  import { api, pageHref, type Calendar, type CalendarPreview } from '../lib/api'
  import { store } from '../lib/store.svelte'

  // The form edits a draft of the world's calendar: time units from the
  // largest down, each a subdivision of the one above, and templates for how
  // dates are written. The server reads the world's dates with the draft as
  // it changes, so every edit shows its effect before saving.

  type UnitDraft = {
    id: number
    name: string
    write: '' | 'roman' | 'names'
    names: string[]
    per: number | undefined
    lengths: { value: string; count: number | undefined }[]
  }

  let nextId = 0
  const blankUnit = (name = ''): UnitDraft => ({ id: nextId++, name, write: '', names: [], per: undefined, lengths: [] })

  function fromCalendar(c: Calendar | undefined) {
    return {
      units: (c?.units ?? []).map((u) => ({
        ...blankUnit(u.name),
        write: u.write === 'roman' || u.write === 'names' ? u.write : '',
        names: [...(u.names ?? [])],
        per: u.per || undefined,
        lengths: Object.entries(u.lengths ?? {}).map(([value, count]) => ({ value, count })),
      })) as UnitDraft[],
      formats: [...(c?.formats ?? [])],
      step: c?.step ?? '',
    }
  }

  function toCalendar(): Calendar {
    return {
      units: draft.units.map((u) => ({
        name: u.name.trim(),
        write: u.write || undefined,
        names: u.write === 'names' ? u.names : undefined,
        per: Number(u.per) || undefined,
        lengths: Object.fromEntries(
          u.lengths.filter((l) => l.value.trim() && Number(l.count) > 0).map((l) => [l.value.trim(), Number(l.count)]),
        ),
      })),
      formats: draft.formats.map((f) => f.trim()).filter(Boolean),
      step: draft.step,
    }
  }

  let draft = $state(fromCalendar(store.world?.calendar))
  // svelte-ignore state_referenced_locally
  let saved = $state(JSON.stringify(toCalendar())) // the starting point, to tell edits from it
  const dirty = $derived(JSON.stringify(toCalendar()) !== saved)
  let saving = $state(false)
  let saveError = $state('')

  document.title = `Календар — ${store.world?.name}`

  // Ready-made starting points.
  const presets: { label: string; build: () => ReturnType<typeof fromCalendar> }[] = [
    {
      label: 'Лише роки',
      build: () => ({ units: [blankUnit('Рік')], formats: ['{Рік}'], step: 'Рік' }),
    },
    {
      label: 'Епохи → роки → частини року',
      build: () => {
        const epoch = { ...blankUnit('Епоха'), write: 'roman' as const }
        const part = { ...blankUnit('Частина'), per: 100 }
        return { units: [epoch, blankUnit('Рік'), part], formats: ['{Рік}[.{Частина}][ р.] {Епоха} Епохи'], step: 'Рік' }
      },
    },
    {
      label: 'Ери → роки → місяці → дні',
      build: () => {
        const era = { ...blankUnit('Ера'), write: 'names' as const, names: ['Стара ера', 'Нова ера'] }
        const month = { ...blankUnit('Місяць'), write: 'names' as const, names: ['Перший місяць', 'Другий місяць'] }
        const day = { ...blankUnit('День'), per: 30 }
        return {
          units: [era, blankUnit('Рік'), month, day],
          formats: ['{День} {Місяць} {Рік} р. {Ера}', '{Місяць} {Рік} р. {Ера}', '{Рік} р. {Ера}'],
          step: 'Рік',
        }
      },
    },
  ]

  // --- checks in the author's terms, before the server's -------------------------

  const problems = $derived.by(() => {
    const out: string[] = []
    if (!draft.units.length) return out
    const names = new Set<string>()
    for (const [i, u] of draft.units.entries()) {
      const name = u.name.trim()
      if (!name) out.push(`Одиниця №${i + 1} не має назви.`)
      else if (names.has(name.toLowerCase())) out.push(`Назва «${name}» повторюється.`)
      else if (/[{}[\]]/.test(name)) out.push(`У назві «${name}» не може бути дужок.`)
      names.add(name.toLowerCase())
      if (u.write === 'names' && !u.names.length) out.push(`«${name || '…'}» пишеться назвами, але їх ще немає.`)
    }
    if (!draft.formats.some((f) => f.trim())) out.push('Додайте хоча б один шаблон запису дат.')
    for (const f of draft.formats) {
      for (const [, token] of f.matchAll(/\{([^}]*)\}/g))
        if (!names.has(token.trim().toLowerCase())) out.push(`Шаблон «${f}» згадує {${token}}, а такої одиниці немає.`)
      if ((f.match(/\[/g) ?? []).length !== (f.match(/\]/g) ?? []).length) out.push(`У шаблоні «${f}» не закрита дужка [ ].`)
    }
    return out
  })

  // --- live preview from the server ----------------------------------------------

  let sample = $state('')
  let preview = $state<CalendarPreview | null>(null)
  let previewTimer: ReturnType<typeof setTimeout>
  $effect(() => {
    const cal = toCalendar()
    const s = sample
    if (problems.length) return
    clearTimeout(previewTimer)
    previewTimer = setTimeout(async () => {
      preview = await api.previewCalendar(cal, s)
    }, 250)
  })

  // --- editing helpers ---------------------------------------------------------------

  function moveUnit(i: number, by: number) {
    const [u] = draft.units.splice(i, 1)
    draft.units.splice(i + by, 0, u)
  }

  // Unit tokens insert into the format field that last had focus.
  let lastFormat = $state(0)
  const formatInputs: HTMLInputElement[] = []
  async function insertToken(name: string) {
    if (!draft.formats.length) draft.formats.push('')
    const i = Math.min(lastFormat, draft.formats.length - 1)
    await tick()
    const el = formatInputs[i]
    if (!el) return
    const at = el.selectionStart ?? el.value.length
    el.setRangeText(`{${name}}`, at, el.selectionEnd ?? at, 'end')
    draft.formats[i] = el.value
    el.focus()
  }

  async function save() {
    saving = true
    saveError = ''
    try {
      store.world = await api.saveCalendar(toCalendar())
      draft = fromCalendar(store.world.calendar)
      saved = JSON.stringify(toCalendar())
    } catch (e) {
      saveError = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  function revert() {
    draft = fromCalendar(store.world?.calendar)
  }

  function clearCalendar() {
    if (confirm('Прибрати календар? Світ знову рахуватиме прості роки (перше число в даті). Не забудьте зберегти.'))
      draft = { units: [], formats: [], step: '' }
  }

  const unitName = (u: UnitDraft | undefined) => u?.name.trim() || '…'
</script>

<svelte:window
  onbeforeunload={(e) => {
    if (dirty) e.preventDefault()
  }}
/>

<header class="top">
  <div>
    <h1 class="page-title">Календар</h1>
    <p class="muted">
      Як у цьому світі рахується час: з яких одиниць він складається і як дати записані в статтях. Без календаря світ
      рахує прості роки — перше число в даті.
    </p>
  </div>
  <div class="actions">
    {#if dirty}<button onclick={revert}>Скасувати зміни</button>{/if}
    <button class="primary" disabled={!dirty || saving || problems.length > 0} onclick={save}>
      {saving ? 'Збереження…' : 'Зберегти'}
    </button>
  </div>
</header>

{#if problems.length || saveError || preview?.error}
  <div class="problems">
    {#each problems as p (p)}<p>{p}</p>{/each}
    {#if !problems.length && preview?.error}<p>Календар не вдається прочитати: {preview.error}</p>{/if}
    {#if saveError}<p>Не вдалося зберегти: {saveError}</p>{/if}
  </div>
{/if}

{#if !draft.units.length}
  <section class="card">
    <h2>Почати</h2>
    <p class="muted">Оберіть заготовку й підлаштуйте її, або додайте одиниці часу самі.</p>
    <div class="presets">
      {#each presets as p (p.label)}
        <button onclick={() => (draft = p.build())}>{p.label}</button>
      {/each}
      <button onclick={() => draft.units.push(blankUnit())}>+ Своя одиниця</button>
    </div>
  </section>
{:else}
  <section class="card">
    <div class="card-head">
      <h2>Одиниці часу</h2>
      <button class="link danger" onclick={clearCalendar}>Прибрати календар</button>
    </div>
    <p class="muted hint">Від найбільшої до найменшої: кожна ділиться на наступну. Наприклад, епоха → рік → місяць → день.</p>

    <ol class="units">
      {#each draft.units as u, i (u.id)}
        {@const parent = draft.units[i - 1]}
        {@const child = draft.units[i + 1]}
        <li class="unit" style:margin-left="{Math.min(i, 4) * 1.1}rem">
          <div class="unit-head">
            <span class="depth">{i + 1}</span>
            <input class="unit-name" bind:value={u.name} placeholder="Назва, напр. Рік" aria-label="Назва одиниці" />
            <button class="icon" disabled={i === 0} onclick={() => moveUnit(i, -1)} aria-label="Вище"><Icon name="up" size={16} /></button>
            <button class="icon" disabled={i === draft.units.length - 1} onclick={() => moveUnit(i, 1)} aria-label="Нижче">
              <Icon name="down" size={16} />
            </button>
            <button class="icon" onclick={() => draft.units.splice(i, 1)} aria-label="Прибрати одиницю"><Icon name="trash" size={16} /></button>
          </div>

          <div class="unit-body">
            <div class="field">
              <span>Як записується</span>
              <div class="segmented" role="radiogroup" aria-label="Як записується">
                <button class:on={u.write === ''} onclick={() => (u.write = '')} role="radio" aria-checked={u.write === ''}>Числом</button>
                <button class:on={u.write === 'roman'} onclick={() => (u.write = 'roman')} role="radio" aria-checked={u.write === 'roman'}>
                  Римськими
                </button>
                <button class:on={u.write === 'names'} onclick={() => (u.write = 'names')} role="radio" aria-checked={u.write === 'names'}>
                  Назвами
                </button>
              </div>
            </div>

            {#if u.write === 'names'}
              <div class="field wide">
                <span>Назви по порядку</span>
                <ChipsInput id="names-{u.id}" bind:values={u.names} placeholder="Напр. Сніжень — Enter, щоб додати" />
              </div>
            {/if}

            {#if parent}
              <label class="field">
                <span>Скільки в одній одиниці «{unitName(parent)}»</span>
                <input
                  type="number"
                  min="1"
                  bind:value={u.per}
                  placeholder={u.write === 'names' && u.names.length ? `${u.names.length} — за кількістю назв` : 'невідомо'}
                />
              </label>
            {/if}

            {#if child}
              <details class="field wide" open={u.lengths.length > 0}>
                <summary>Особливі значення: скільки «{unitName(child)}» у конкретних «{unitName(u)}»</summary>
                {#each u.lengths as l, k (k)}
                  <div class="length">
                    {#if u.write === 'names'}
                      <select bind:value={l.value} aria-label="Значення">
                        <option value="">—</option>
                        {#each u.names as n (n)}<option value={n}>{n}</option>{/each}
                      </select>
                    {:else}
                      <input bind:value={l.value} placeholder={u.write === 'roman' ? 'Напр. I' : 'Напр. 3'} aria-label="Значення" />
                    {/if}
                    <span class="muted">→</span>
                    <input type="number" min="1" bind:value={l.count} placeholder="кількість" aria-label="Кількість" />
                    <span class="muted">«{unitName(child)}»</span>
                    <button class="icon" onclick={() => u.lengths.splice(k, 1)} aria-label="Прибрати"><Icon name="trash" size={15} /></button>
                  </div>
                {/each}
                <button class="link" onclick={() => u.lengths.push({ value: '', count: undefined })}>+ Додати</button>
                <p class="muted hint">
                  Напр. «I Епоха — 500 років» або «Талець — 28 днів». Коли відомо, скільки триває кожна велика одиниця, таймлайн
                  кладе їх на одну шкалу; інакше кожна отримує свою вкладку.
                </p>
              </details>
            {/if}
          </div>
        </li>
      {/each}
    </ol>
    <button onclick={() => draft.units.push(blankUnit())}>+ Додати одиницю</button>

    <label class="field step">
      <span>Таймлайн крокує одиницею</span>
      <select bind:value={draft.step}>
        <option value="">автоматично</option>
        {#each draft.units as u (u.id)}{#if u.name.trim()}<option value={u.name.trim()}>{u.name.trim()}</option>{/if}{/each}
      </select>
      <span class="muted">Одна точка на таймлайні — одна така одиниця (зазвичай рік).</span>
    </label>
  </section>

  <section class="card">
    <h2>Як записані дати</h2>
    <p class="muted hint">
      Шаблони, за якими програма шукає дати в полях-датах. <code>{'{Назва}'}</code> — значення одиниці,
      <code>[ … ]</code> — необов'язкова частина. Діапазон — дві дати через тире; у першій можна не повторювати те, що
      записано в другій: «144.087 — 144.125 III Епохи».
    </p>
    <ol class="formats">
      {#each draft.formats as _, i (i)}
        <li>
          <input
            bind:this={formatInputs[i]}
            bind:value={draft.formats[i]}
            onfocus={() => (lastFormat = i)}
            placeholder={'Напр. {Рік}[.{Частина}] {Епоха} Епохи'}
            aria-label="Шаблон дати"
          />
          {#if preview && !preview.error && preview.formats[i] !== undefined}
            <span class="count muted">читає {preview.formats[i]}</span>
          {/if}
          <button class="icon" onclick={() => draft.formats.splice(i, 1)} aria-label="Прибрати шаблон"><Icon name="trash" size={16} /></button>
        </li>
      {/each}
    </ol>
    <div class="tokens">
      <button onclick={() => draft.formats.push('')}>+ Додати шаблон</button>
      <span class="muted">Вставити:</span>
      {#each draft.units as u (u.id)}
        {#if u.name.trim()}<button class="token" onclick={() => insertToken(u.name.trim())}>{'{'}{u.name.trim()}{'}'}</button>{/if}
      {/each}
    </div>
  </section>
{/if}

<section class="card">
  <h2>Перевірка</h2>
  <label class="field wide">
    <span>Будь-яка дата</span>
    <input bind:value={sample} placeholder="Напр. 144.087 — 144.125 III Епохи" />
  </label>
  {#if preview?.test}
    {#if preview.test.ok}
      <div class="parsed">
        {#each preview.test.start ?? [] as p (p.unit)}<span class="part"><b>{p.unit}</b> {p.value}</span>{/each}
        {#if preview.test.end}
          <span class="muted">—</span>
          {#each preview.test.end as p (p.unit)}<span class="part"><b>{p.unit}</b> {p.value}</span>{/each}
        {/if}
      </div>
      {#if preview.test.label}<p class="muted hint">На таймлайні: «{preview.test.label}»</p>{/if}
    {:else}
      <p class="warn">Жоден шаблон не підходить до цієї дати.</p>
    {/if}
  {/if}

  {#if preview && !preview.error}
    <h3>Що вийде для цього світу</h3>
    <ul class="summary">
      <li>
        Прочитано дат: <b>{preview.read}</b> із {preview.read + preview.unread.length}.
      </li>
      {#if preview.points}
        <li>
          Таймлайн:
          {#if preview.axes > 1}окремі вкладки ({preview.axes}) — тривалість старших одиниць невідома
          {:else if preview.shared}одна спільна шкала з межами
          {:else}одна шкала{/if}.
        </li>
      {/if}
      {#if preview.now}
        <li>
          Теперішній момент — найпізніша дата: <b>{preview.now}</b>
          {#if preview.nowFrom}<span class="muted">(<a href={pageHref(preview.nowFrom.title)}>{preview.nowFrom.title}</a>, {preview.nowFrom.field.toLowerCase()})</span>{/if}
        </li>
      {/if}
    </ul>
    {#if preview.unread.length}
      <details open={preview.unread.length <= 8}>
        <summary class="warn">Не вдалося прочитати ({preview.unread.length}) — на таймлайн не потраплять</summary>
        <ul class="refs">
          {#each preview.unread as r (r.title + r.field)}
            <li><a href={pageHref(r.title)}>{r.title}</a> <span class="muted">· {r.field.toLowerCase()}: «{r.date}»</span></li>
          {/each}
        </ul>
      </details>
    {/if}
    {#if preview.beyond.length}
      <details open>
        <summary class="warn">Не вкладаються в календар ({preview.beyond.length})</summary>
        <ul class="refs">
          {#each preview.beyond as r (r.title + r.field)}
            <li><a href={pageHref(r.title)}>{r.title}</a> <span class="muted">· {r.field.toLowerCase()}: «{r.date}»</span></li>
          {/each}
        </ul>
        <p class="muted hint">Значення більше, ніж дозволяє батьківська одиниця. На таймлайні такі дати стоять у кінці свого кроку.</p>
      </details>
    {/if}
  {/if}
</section>

<style>
  .top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
    max-width: 56rem;
  }
  .top p {
    margin: 0.2rem 0 0;
    max-width: 40rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .problems {
    max-width: 56rem;
    margin-top: 1rem;
    padding: 0.5rem 0.9rem;
    border-radius: var(--radius);
    color: var(--danger);
    background: color-mix(in srgb, var(--danger) 8%, transparent);
    font-size: 0.9rem;
  }
  .problems p {
    margin: 0.2rem 0;
  }
  .card {
    max-width: 56rem;
    margin-top: 1.25rem;
    padding: 1.1rem 1.3rem 1.2rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-raised);
  }
  .card-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }
  h2 {
    margin: 0 0 0.5rem;
    font-size: 1.1rem;
  }
  h3 {
    margin: 1.2rem 0 0.4rem;
    font-size: 0.95rem;
  }
  .hint {
    font-size: 0.82rem;
    margin: 0.4rem 0;
  }
  code {
    font-family: var(--mono);
    font-size: 0.9em;
  }
  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .units {
    list-style: none;
    margin: 0.8rem 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .unit {
    border: 1px solid var(--border);
    border-left: 3px solid var(--accent);
    border-radius: 8px;
    padding: 0.6rem 0.8rem 0.7rem;
  }
  .unit-head {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .depth {
    display: grid;
    place-items: center;
    width: 1.6rem;
    height: 1.6rem;
    border-radius: 50%;
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 700;
    font-size: 0.85rem;
    flex: none;
  }
  .unit-name {
    flex: 1;
    min-width: 0;
    font-weight: 600;
    padding: 0.3rem 0.55rem;
  }
  .unit-body {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem 1.2rem;
    margin-top: 0.6rem;
    padding-left: 2rem;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.85rem;
  }
  .field > span,
  summary {
    font-weight: 600;
  }
  .field.wide {
    flex-basis: 100%;
  }
  .field input,
  .field select,
  .length input,
  .length select {
    padding: 0.3rem 0.5rem;
  }
  input[type='number'] {
    width: 11rem;
  }
  summary {
    cursor: pointer;
  }
  .length {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.35rem;
  }
  .length input:not([type='number']),
  .length select {
    width: 10rem;
  }
  .length input[type='number'] {
    width: 7rem;
  }
  .segmented {
    display: inline-flex;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }
  .segmented button {
    border: none;
    border-radius: 0;
    padding: 0.25rem 0.65rem;
    font-size: 0.85rem;
    background: none;
  }
  .segmented button + button {
    border-left: 1px solid var(--border);
  }
  .segmented button.on {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .step {
    flex-direction: row;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  .step span:last-child {
    font-weight: 400;
  }

  .formats {
    list-style: none;
    margin: 0.6rem 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .formats li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .formats input {
    flex: 1;
    min-width: 0;
    padding: 0.35rem 0.6rem;
    font-family: var(--mono);
    font-size: 0.9rem;
  }
  .count {
    font-size: 0.8rem;
    white-space: nowrap;
  }
  .tokens {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
  }
  .token {
    padding: 0.15rem 0.5rem;
    font-family: var(--mono);
    font-size: 0.82rem;
    color: var(--accent);
  }
  .icon {
    display: grid;
    place-items: center;
    padding: 0.3rem;
    border-color: transparent;
    background: none;
    color: var(--text-muted);
  }
  .link {
    border: none;
    background: none;
    padding: 0;
    color: var(--accent);
    font-size: 0.85rem;
  }
  .link:hover {
    background: none;
    text-decoration: underline;
  }
  .danger {
    color: var(--danger);
  }

  .parsed {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.35rem;
    margin-top: 0.6rem;
  }
  .part {
    padding: 0.15rem 0.6rem;
    border-radius: 999px;
    background: var(--accent-soft);
    font-size: 0.85rem;
  }
  .part b {
    color: var(--accent);
    margin-right: 0.2rem;
  }
  .warn {
    color: var(--danger);
  }
  .summary,
  .refs {
    margin: 0.3rem 0;
    padding-left: 1.2rem;
    font-size: 0.9rem;
  }
  details {
    margin-top: 0.6rem;
  }
</style>
