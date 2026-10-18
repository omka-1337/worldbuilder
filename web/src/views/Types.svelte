<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import { api, type TypeInfo } from '../lib/api'
  import { store, refresh } from '../lib/store.svelte'
  import { plural } from '../lib/format'
  import { palette } from '../lib/colors'

  // Edits are made on a local copy and written to the world in one save.
  let types = $state<TypeInfo[]>([])
  let saved = $state('')
  let selected = $state(0)
  let saving = $state(false)
  let error = $state('')
  let nameInput = $state<HTMLInputElement>()

  const dirty = $derived(JSON.stringify(types) !== saved)
  const current = $derived(types[selected] as TypeInfo | undefined)

  document.title = `Типи статей — ${store.world?.name}`
  api.types().then(reset)

  function reset(list: TypeInfo[]) {
    types = list
    saved = JSON.stringify(list)
    selected = Math.min(selected, Math.max(list.length - 1, 0))
    error = ''
  }

  // Problems shown before saving, in the user's terms.
  const problems = $derived.by(() => {
    const out: string[] = []
    const labels = new Set<string>()
    for (const t of types) {
      const name = t.label.trim()
      if (!name) out.push('Один із типів не має назви.')
      else if (labels.has(name.toLowerCase())) out.push(`Назва «${name}» повторюється.`)
      labels.add(name.toLowerCase())
      if (t.fields.some((f) => !f.label.trim())) out.push(`Тип «${name || 'без назви'}» має поле без назви.`)
    }
    return out
  })

  async function addType() {
    // New types take the first palette color nobody uses yet.
    const used = new Set(types.map((t) => t.color))
    const color = palette.find((p) => !used.has(p.color))?.color ?? ''
    types.push({ key: '', label: '', color, event: false, fields: [{ key: '', label: '', type: '' }], count: 0 })
    selected = types.length - 1
    await tick()
    nameInput?.focus()
  }

  function removeType() {
    const t = current
    if (!t) return
    const used = t.count
      ? `\n\n${plural(t.count, ['стаття має', 'статті мають', 'статей мають'])} цей тип. ` +
        'Вони залишаться, а заповнені поля показуватимуться без підписів типу.'
      : ''
    if (!confirm(`Видалити тип «${t.label || 'без назви'}»?${used}`)) return
    types.splice(selected, 1)
    selected = Math.max(0, selected - 1)
  }

  function moveField(i: number, by: number) {
    const fields = current!.fields
    const [f] = fields.splice(i, 1)
    fields.splice(i + by, 0, f)
  }

  async function save() {
    if (problems.length || saving) return
    saving = true
    error = ''
    try {
      // Keep the same type selected; a new one is matched by its label.
      const key = current?.key
      const label = current?.label.trim()
      const fresh = await api.saveTypes(types)
      reset(fresh)
      const i = fresh.findIndex((t) => (key ? t.key === key : t.label === label))
      if (i >= 0) selected = i
      await refresh()
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }
</script>

<svelte:window
  onbeforeunload={(e) => {
    if (dirty) e.preventDefault()
  }}
/>

<header class="top">
  <div>
    <h1 class="page-title">Типи статей</h1>
    <p class="muted">Тип визначає, які поля є в інфобоксі статті. Наприклад, у персонажа — дата народження і раса.</p>
  </div>
  <div class="actions">
    {#if dirty}
      <button onclick={() => reset(JSON.parse(saved))}>Скасувати зміни</button>
    {/if}
    <button class="primary" disabled={!dirty || saving || problems.length > 0} onclick={save}>
      {saving ? 'Збереження…' : 'Зберегти'}
    </button>
  </div>
</header>

{#if problems.length || error}
  <div class="problems">
    {#each problems as p (p)}<p>{p}</p>{/each}
    {#if error}<p>Не вдалося зберегти: {error}</p>{/if}
  </div>
{/if}

<div class="editor">
  <nav class="list" aria-label="Типи">
    {#each types as t, i (i)}
      <button class="item" class:active={i === selected} onclick={() => (selected = i)}>
        <span class="name" class:muted={!t.label.trim()}>
          <span class="swatch-dot" style:background={t.color || 'var(--accent)'}></span>
          {t.label.trim() || 'Без назви'}
        </span>
        <span class="muted small">
          {t.count ? plural(t.count, ['стаття', 'статті', 'статей']) : 'не використовується'}
          {#if t.event}· подія{/if}
        </span>
      </button>
    {/each}
    <button class="add" onclick={addType}>+ Новий тип</button>
  </nav>

  {#if current}
    <section class="detail">
      <label class="field-name">
        <span>Назва типу</span>
        <input bind:this={nameInput} bind:value={current.label} placeholder="Наприклад, Персонаж" />
      </label>

      <div class="color">
        <span class="color-label">Колір</span>
        <div class="swatches" role="radiogroup" aria-label="Колір типу">
          {#each palette as p (p.color)}
            <button
              class="swatch"
              class:on={current.color === p.color}
              style:background={p.color}
              onclick={() => (current.color = p.color)}
              role="radio"
              aria-checked={current.color === p.color}
              aria-label={p.name}
              title={p.name}
            ></button>
          {/each}
          <label class="swatch custom" class:on={!!current.color && !palette.some((p) => p.color === current!.color)} title="Свій колір">
            <input type="color" bind:value={current.color} aria-label="Свій колір" />
          </label>
        </div>
        <span class="muted small">Цим кольором тип показується на таймлайні.</span>
      </div>

      <label class="check">
        <input type="checkbox" bind:checked={current.event} />
        <span>
          Статті цього типу — події
          <span class="muted small">На таймлайні подія дає назву своєму року.</span>
        </span>
      </label>

      <h2>Поля інфобокса</h2>
      {#if current.fields.length}
        <ol class="fields">
          {#each current.fields as f, i (i)}
            <li>
              <input bind:value={f.label} placeholder="Назва поля" aria-label="Назва поля" />
              <div class="kind" role="radiogroup" aria-label="Вид поля">
                <button class:on={f.type !== 'date'} onclick={() => (f.type = '')} role="radio" aria-checked={f.type !== 'date'}>
                  Текст
                </button>
                <button class:on={f.type === 'date'} onclick={() => (f.type = 'date')} role="radio" aria-checked={f.type === 'date'}>
                  Дата
                </button>
              </div>
              <button class="icon" disabled={i === 0} onclick={() => moveField(i, -1)} aria-label="Вище"><Icon name="up" size={16} /></button>
              <button class="icon" disabled={i === current.fields.length - 1} onclick={() => moveField(i, 1)} aria-label="Нижче">
                <Icon name="down" size={16} />
              </button>
              <button class="icon" onclick={() => current.fields.splice(i, 1)} aria-label="Видалити поле"><Icon name="trash" size={16} /></button>
            </li>
          {/each}
        </ol>
      {:else}
        <p class="muted">Полів ще немає.</p>
      {/if}
      <button onclick={() => current.fields.push({ key: '', label: '', type: '' })}>+ Додати поле</button>
      <p class="muted small">Поля з видом «Дата» потрапляють на таймлайн. Порядок полів — це порядок рядків в інфобоксі.</p>

      <footer>
        <button class="danger" onclick={removeType}><Icon name="trash" size={16} /> Видалити тип</button>
      </footer>
    </section>
  {:else}
    <section class="detail empty">
      <p class="muted">Типів ще немає. Створіть перший, наприклад «Персонаж» або «Місто».</p>
    </section>
  {/if}
</div>

<style>
  .top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
    max-width: 60rem;
  }
  .top p {
    margin: 0.2rem 0 0;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .problems {
    max-width: 60rem;
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

  .editor {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    gap: 1.5rem;
    max-width: 60rem;
    margin-top: 1.5rem;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .item {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    text-align: left;
    border-color: transparent;
    background: none;
    padding: 0.45rem 0.7rem;
  }
  .item.active {
    background: var(--accent-soft);
    border-color: transparent;
  }
  .item.active .name {
    color: var(--accent);
  }
  .name {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-weight: 600;
  }
  .swatch-dot {
    width: 0.7rem;
    height: 0.7rem;
    border-radius: 50%;
    flex: none;
  }
  .color {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin-top: 1rem;
  }
  .color-label {
    font-weight: 600;
    font-size: 0.9rem;
  }
  .swatches {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .swatch {
    width: 1.7rem;
    height: 1.7rem;
    padding: 0;
    border-radius: 50%;
    border: 2px solid var(--bg-raised);
    box-shadow: 0 0 0 1px var(--border);
    cursor: pointer;
  }
  .swatch:hover {
    transform: scale(1.1);
  }
  .swatch.on {
    box-shadow: 0 0 0 2px var(--text);
  }
  .swatch.custom {
    position: relative;
    overflow: hidden;
    background: conic-gradient(#d9534f, #c9a227, #3f9b5c, #3b82c4, #8e5bd4, #d9534f);
  }
  .swatch.custom input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }
  .small {
    font-size: 0.8rem;
  }
  .add {
    margin-top: 0.4rem;
    border-style: dashed;
    background: none;
    color: var(--accent);
  }

  .detail {
    padding: 1.25rem 1.4rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-raised);
  }
  .detail.empty {
    display: grid;
    place-items: center;
    min-height: 12rem;
  }
  .field-name {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    font-weight: 600;
    font-size: 0.9rem;
  }
  .field-name input {
    font-size: 1.1rem;
    font-weight: 600;
    padding: 0.45rem 0.7rem;
  }
  .check {
    display: flex;
    gap: 0.6rem;
    align-items: flex-start;
    margin-top: 1rem;
  }
  .check input {
    margin-top: 0.35rem;
  }
  .check .small {
    display: block;
  }
  h2 {
    font-size: 1rem;
    margin: 1.5rem 0 0.6rem;
  }
  .fields {
    list-style: none;
    margin: 0 0 0.6rem;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .fields li {
    display: flex;
    gap: 0.35rem;
    align-items: center;
  }
  .fields input {
    flex: 1;
    min-width: 0;
    padding: 0.35rem 0.6rem;
  }
  .kind {
    display: inline-flex;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }
  .kind button {
    border: none;
    border-radius: 0;
    padding: 0.3rem 0.65rem;
    font-size: 0.85rem;
    background: none;
  }
  .kind button + button {
    border-left: 1px solid var(--border);
  }
  .kind button.on {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .icon {
    display: grid;
    place-items: center;
    padding: 0.3rem;
    border-color: transparent;
    background: none;
    color: var(--text-muted);
  }
  .icon:hover:not(:disabled) {
    color: var(--text);
  }
  footer {
    margin-top: 1.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border);
  }
  .danger {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--danger);
  }

  @media (max-width: 800px) {
    .editor {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
