<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import { api, pageHref, type Calendar, type TimelineYear, type World } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { epochName, epochOffsets, outOfCalendar, presentMoment, roman, yearLabel } from '../lib/calendar'

  // A draft of the world's calendar, saved in one go.
  const clone = (c: Calendar | undefined): Calendar => ({
    yearParts: c?.yearParts || undefined,
    partName: c?.partName ?? '',
    epochs: (c?.epochs ?? []).map((e) => ({ name: e.name ?? '', years: e.years || undefined })),
  })
  let draft = $state<Calendar>(clone(store.world?.calendar))
  let era = $state(store.world?.era ?? '')
  // svelte-ignore state_referenced_locally
  let saved = $state(JSON.stringify({ draft, era })) // the starting point, to tell edits from it
  let saving = $state(false)
  let error = $state('')
  let timeline = $state<TimelineYear[]>([])

  document.title = `Календар — ${store.world?.name}`
  api.timeline().then((t) => (timeline = t))

  const dirty = $derived(JSON.stringify({ draft, era }) !== saved)
  // The world as it would be with the draft, for previews.
  const preview = $derived<World | null>(store.world ? { ...store.world, era, calendar: draft } : null)

  const inUse = $derived([...new Set(timeline.map((y) => y.epoch ?? 0))].filter((e) => e > 0).sort((a, b) => a - b))
  const shared = $derived(!!epochOffsets(preview, inUse))
  const missingLengths = $derived(
    inUse.length > 1 ? Array.from({ length: Math.max(...inUse) - 1 }, (_, i) => i + 1).filter((e) => !draft.epochs[e - 1]?.years) : [],
  )
  const present = $derived(presentMoment(timeline))
  const presentEntry = $derived.by(() => {
    if (!present) return null
    for (const y of timeline)
      for (const e of y.entries) {
        if ((e.epoch ?? 0) === present.epoch && e.year === present.year && (e.part ?? 0) === present.part) return e
        if (e.endYear === present.year && (e.endEpoch ?? e.epoch ?? 0) === present.epoch && (e.endPart ?? 0) === present.part)
          return { ...e, field: e.endField ?? e.field, date: e.endDate ?? e.date }
      }
    return null
  })
  const problems = $derived(outOfCalendar(preview, timeline))

  // Epochs in the dates that the list does not have yet.
  const unlisted = $derived(inUse.filter((e) => e > draft.epochs.length))

  function addEpoch() {
    draft.epochs.push({ name: '', years: undefined })
  }

  async function save() {
    saving = true
    error = ''
    try {
      const cal: Calendar = {
        yearParts: Number(draft.yearParts) || 0,
        partName: draft.partName?.trim() ?? '',
        epochs: draft.epochs.map((e) => ({ name: e.name?.trim() ?? '', years: Number(e.years) || 0 })),
      }
      store.world = await api.saveCalendar(cal, era.trim())
      draft = clone(store.world.calendar)
      era = store.world.era ?? ''
      saved = JSON.stringify({ draft, era })
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  function revert() {
    const s = JSON.parse(saved)
    draft = s.draft
    era = s.era
  }

  const ordinal = (n: number) => `${n}-${n % 10 === 3 && n % 100 !== 13 ? 'тя' : 'та'}`
</script>

<svelte:window
  onbeforeunload={(e) => {
    if (dirty) e.preventDefault()
  }}
/>

<header class="top">
  <div>
    <h1 class="page-title">Календар</h1>
    <p class="muted">Як у цьому світі рахується час. Усе необов'язкове: можна заповнювати поступово.</p>
  </div>
  <div class="actions">
    {#if dirty}<button onclick={revert}>Скасувати зміни</button>{/if}
    <button class="primary" disabled={!dirty || saving} onclick={save}>{saving ? 'Збереження…' : 'Зберегти'}</button>
  </div>
</header>
{#if error}<p class="error">Не вдалося зберегти: {error}</p>{/if}

<section class="card">
  <h2>Рік</h2>
  <div class="row">
    <label>
      <span>Частин у році</span>
      <input type="number" min="1" bind:value={draft.yearParts} placeholder="не задано" />
    </label>
    <label class="grow">
      <span>Як називається частина</span>
      <input bind:value={draft.partName} placeholder="Напр. день або цикл — можна лишити порожнім" />
    </label>
  </div>
  <p class="hint muted">
    Частина — це число після крапки в даті.
    {#if draft.yearParts}
      Наприклад, <code>139.096</code> — це {ordinal(96)} {draft.partName?.trim() || 'частина'} з {draft.yearParts} у 139-му році.
    {:else}
      Наприклад, у <code>139.096</code> це 96. Поки кількість не задано, частина лише впорядковує події всередині року.
    {/if}
  </p>
  {#if problems.length}
    <div class="problems">
      <strong>Ці дати не вкладаються в рік із {draft.yearParts} частин:</strong>
      <ul>
        {#each problems as p (p.title + p.field)}
          <li><a href={pageHref(p.title)}>{p.title}</a> <span class="muted">· {p.field.toLowerCase()}: {p.date}</span></li>
        {/each}
      </ul>
      <span class="muted">На таймлайні вони стоять наприкінці свого року.</span>
    </div>
  {/if}
</section>

<section class="card">
  <h2>Епохи</h2>
  <label class="era">
    <span>Підпис після номера епохи в датах</span>
    <input bind:value={era} placeholder="Напр. Епохи" />
    <span class="muted">→ {yearLabel(preview, Math.max(draft.epochs.length, 1), 139)}</span>
  </label>

  {#if draft.epochs.length}
    <ol class="epochs">
      {#each draft.epochs as e, i (i)}
        <li>
          <span class="num">{roman(i + 1)}</span>
          <input bind:value={e.name} placeholder={`Назва (необов'язково) — інакше «${roman(i + 1)}${era ? ' ' + era : ''}»`} aria-label="Назва епохи" />
          <label class="years">
            <input type="number" min="1" bind:value={e.years} placeholder="?" aria-label="Тривалість у роках" />
            <span class="muted">років</span>
          </label>
          {#if i === draft.epochs.length - 1}
            <button class="icon" onclick={() => draft.epochs.pop()} aria-label="Прибрати епоху"><Icon name="trash" size={16} /></button>
          {:else}
            <span class="icon-space"></span>
          {/if}
        </li>
      {/each}
    </ol>
  {/if}
  <button onclick={addEpoch}>+ Додати епоху</button>
  {#if unlisted.length}
    <p class="hint muted">
      У датах світу трапляються епохи {unlisted.map(roman).join(', ')}, яких ще немає в списку.
    </p>
  {/if}
  <p class="hint muted">
    Роки в кожній епосі рахуються заново. Якщо відомо, скільки тривали всі епохи, крім останньої, таймлайн покаже їх на
    одній спільній шкалі з межами епох; інакше кожна епоха матиме свою вкладку.
    {#if inUse.length > 1}
      <br /><strong>Зараз:</strong>
      {#if shared}спільна шкала.{:else}окремі вкладки — не вистачає тривалості {missingLengths.map((e) => epochName(preview, e)).join(', ')}.{/if}
    {/if}
  </p>
</section>

<section class="card">
  <h2>Теперішній момент</h2>
  {#if present}
    <p>
      Найпізніша дата у світі: <strong>{yearLabel(preview, present.epoch, present.year)}</strong>
      {#if presentEntry}<span class="muted">— <a href={pageHref(presentEntry.title)}>{presentEntry.title}</a>, {presentEntry.field.toLowerCase()}: {presentEntry.date}</span>{/if}
    </p>
    <p class="hint muted">
      Вона вважається «зараз»: відкриті періоди (живі персонажі, держави, що досі існують) тягнуться на таймлайні до неї.
    </p>
  {:else}
    <p class="muted">Дат на таймлайні ще немає.</p>
  {/if}
</section>

<style>
  .top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
    max-width: 52rem;
  }
  .top p {
    margin: 0.2rem 0 0;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .error {
    color: var(--danger);
  }
  .card {
    max-width: 52rem;
    margin-top: 1.25rem;
    padding: 1.1rem 1.3rem 1.2rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-raised);
  }
  h2 {
    margin: 0 0 0.8rem;
    font-size: 1.1rem;
  }
  .row {
    display: flex;
    gap: 1rem;
    flex-wrap: wrap;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.88rem;
    font-weight: 600;
  }
  label.grow {
    flex: 1;
    min-width: 14rem;
  }
  input {
    padding: 0.35rem 0.6rem;
    font-weight: 400;
  }
  input[type='number'] {
    width: 9rem;
  }
  .hint {
    font-size: 0.82rem;
    margin: 0.7rem 0 0;
  }
  code {
    font-family: var(--mono);
  }
  .problems {
    margin-top: 0.8rem;
    padding: 0.6rem 0.9rem;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--danger) 8%, transparent);
    font-size: 0.88rem;
  }
  .problems strong {
    color: var(--danger);
  }
  .problems ul {
    margin: 0.3rem 0;
    padding-left: 1.2rem;
  }
  .era {
    flex-direction: row;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
    margin-bottom: 0.9rem;
  }
  .era span:last-child {
    font-weight: 400;
  }
  .epochs {
    list-style: none;
    margin: 0 0 0.6rem;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .epochs li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .epochs li > input {
    flex: 1;
    min-width: 0;
  }
  .num {
    width: 2.6rem;
    font-weight: 700;
    font-family: Georgia, serif;
    text-align: right;
  }
  .years {
    flex-direction: row;
    align-items: center;
    gap: 0.35rem;
    font-weight: 400;
  }
  .years input {
    width: 6rem;
  }
  .icon,
  .icon-space {
    width: 2rem;
    flex: none;
  }
  .icon {
    display: grid;
    place-items: center;
    padding: 0.3rem;
    border-color: transparent;
    background: none;
    color: var(--text-muted);
  }
</style>
