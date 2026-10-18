<script lang="ts">
  import { onMount, tick } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import ChipsInput from '../components/ChipsInput.svelte'
  import { api, pageHref, worldHref, type FieldDef } from '../lib/api'
  import { navigate } from '../lib/router.svelte'
  import { store, refresh } from '../lib/store.svelte'

  let { title }: { title: string } = $props()

  let isNew = $state(false)
  let loading = $state(true)
  let saving = $state(false)
  let error = $state('')

  // The form. Infobox values are kept by field key, so switching the type
  // back and forth does not lose what was typed.
  let type = $state('')
  let values = $state<Record<string, string>>({})
  let extras = $state<string[]>([]) // fields of this article only, not of its type
  let aliases = $state<string[]>([])
  let categories = $state<string[]>([])
  let body = $state('')

  let loadedType = ''
  let saved = $state('')
  let allCategories = $state<string[]>([])
  let newField = $state('')
  let addingField = $state(false)
  let newFieldInput = $state<HTMLInputElement>()

  const types = $derived(Object.entries(store.world?.types ?? {}).sort((a, b) => a[1].label.localeCompare(b[1].label)))
  const schema = $derived<FieldDef[]>(store.world?.types[type]?.fields ?? [])
  const era = $derived(store.world?.era ? ' ' + store.world.era : '')

  onMount(async () => {
    document.title = `Редагування: ${title} — ${store.world?.name}`
    api.categories().then((c) => (allCategories = c.map((x) => x.name)))
    try {
      const src = await api.source(title)
      isNew = !src.exists
      type = loadedType = src.type
      aliases = src.aliases
      categories = src.categories
      body = src.exists ? src.body : `**${title}** — \n`
      const known = new Set((store.world?.types[src.type]?.fields ?? []).map((f) => f.key))
      for (const f of src.infobox) {
        values[f.key] = f.value
        if (!known.has(f.key)) extras.push(f.key)
      }
      saved = JSON.stringify(build())
    } catch (e) {
      error = String(e)
    }
    loading = false
  })

  function build() {
    const infobox = [...schema.map((f) => f.key), ...extras].map((key) => ({ key, value: values[key] ?? '' }))
    return { type, aliases, categories, infobox, body }
  }

  const dirty = $derived(!loading && JSON.stringify(build()) !== saved)

  // Values typed into the old type's fields go away if the type changes.
  const dropped = $derived.by(() => {
    if (type === loadedType) return []
    const now = new Set([...schema.map((f) => f.key), ...extras])
    return (store.world?.types[loadedType]?.fields ?? []).filter((f) => !now.has(f.key) && values[f.key]?.trim())
  })

  async function addField(e: SubmitEvent) {
    e.preventDefault()
    const key = newField.trim()
    if (!key) return
    if (!extras.includes(key) && !schema.some((f) => f.key === key)) extras.push(key)
    newField = ''
    addingField = false
    await tick()
    document.getElementById(`field-${key}`)?.focus()
  }

  async function startAddingField() {
    addingField = true
    await tick()
    newFieldInput?.focus()
  }

  async function save() {
    if (saving || loading) return
    saving = true
    error = ''
    try {
      const newTitle = await api.saveSource(title, build())
      saved = JSON.stringify(build())
      await refresh()
      navigate(pageHref(newTitle), { replace: true })
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  function cancel() {
    if (dirty && !confirm('Відкинути незбережені зміни?')) return
    saved = JSON.stringify(build())
    if (isNew) history.back()
    else navigate(pageHref(title))
  }

  const hasYear = (v: string) => /\d/.test(v)
</script>

<svelte:window
  onkeydown={(e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault()
      save()
    }
  }}
  onbeforeunload={(e) => {
    if (dirty) e.preventDefault()
  }}
/>

<header class="top">
  <h1 class="page-title">
    <span class="muted">{isNew ? 'Створення:' : 'Редагування:'}</span>
    {title}
  </h1>
  <div class="actions">
    <button onclick={cancel}>Скасувати</button>
    <button class="primary" onclick={save} disabled={saving || loading}>
      {saving ? 'Збереження…' : 'Зберегти'}
    </button>
  </div>
</header>

{#if error}<p class="error">Не вдалося зберегти: {error}</p>{/if}

{#if !loading}
  <div class="editor">
    <section class="text">
      <label for="body" class="label">Текст статті</label>
      <!-- svelte-ignore a11y_autofocus -->
      <textarea id="body" bind:value={body} spellcheck="true" autofocus={!isNew}></textarea>
      <p class="help muted">
        <code>[[Стаття]]</code> — посилання, <code>[[Стаття|текст]]</code> — посилання з іншим текстом,
        <code>## Розділ</code> — заголовок розділу, <code>**жирний**</code>, <code>*курсив*</code>. Ctrl+S — зберегти.
      </p>
    </section>

    <aside class="props">
      <div class="group">
        <div class="row-head">
          <label for="type" class="label">Тип статті</label>
          <a class="manage" href={worldHref(undefined, '/types')}>Керувати типами</a>
        </div>
        <select id="type" bind:value={type}>
          <option value="">Без типу</option>
          {#each types as [key, def] (key)}<option value={key}>{def.label}</option>{/each}
        </select>
        {#if dropped.length}
          <p class="warn">
            Після збереження зникнуть поля попереднього типу: {dropped.map((f) => f.label).join(', ')}.
          </p>
        {/if}
      </div>

      {#if schema.length || extras.length}
        <div class="group">
          <span class="label">Інфобокс</span>
          {#each schema as f (f.key)}
            <label class="field">
              <span>{f.label}</span>
              <input
                id="field-{f.key}"
                bind:value={values[f.key]}
                placeholder={f.type === 'date' ? `Рік, напр. 343${era} або 340–347` : ''}
              />
              {#if f.type === 'date' && values[f.key]?.trim() && !hasYear(values[f.key])}
                <small class="warn">Без числа року стаття не з'явиться на таймлайні.</small>
              {/if}
            </label>
          {/each}
          {#each extras as key, i (key)}
            <div class="field">
              <label for="field-{key}">{key === 'image' ? 'Зображення' : key}</label>
              <div class="with-remove">
                <input id="field-{key}" bind:value={values[key]} />
                <button class="icon" onclick={() => extras.splice(i, 1)} aria-label="Прибрати поле «{key}»">
                  <Icon name="trash" size={15} />
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}

      <div class="group">
        {#if addingField}
          <form class="add-field" onsubmit={addField}>
            <input bind:this={newFieldInput} bind:value={newField} placeholder="Назва поля" aria-label="Назва нового поля" />
            <button class="primary" disabled={!newField.trim()}>Додати</button>
            <button type="button" onclick={() => (addingField = false)}>Скасувати</button>
          </form>
        {:else}
          <button class="link" onclick={startAddingField}>+ Своє поле для цієї статті</button>
        {/if}
      </div>

      <div class="group">
        <label for="aliases" class="label">Інші назви</label>
        <ChipsInput id="aliases" bind:values={aliases} placeholder="Напр. прізвисько" />
        <small class="muted">Посилання на ці назви відкриватимуть цю статтю.</small>
      </div>

      <div class="group">
        <label for="categories" class="label">Категорії</label>
        <ChipsInput id="categories" bind:values={categories} suggestions={allCategories} placeholder="Напр. Персонажі" />
      </div>
    </aside>
  </div>
{/if}

<style>
  .top {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
    flex-wrap: wrap;
    max-width: 76rem;
  }
  .page-title {
    font-size: 1.5rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .error {
    color: var(--danger);
    white-space: pre-wrap;
  }

  .editor {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 20rem;
    gap: 1.5rem;
    max-width: 76rem;
    margin-top: 1rem;
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    min-height: calc(100vh - 9rem);
  }
  textarea {
    flex: 1;
    min-height: 20rem;
    padding: 1rem 1.1rem;
    font: 1rem/1.7 var(--sans);
    resize: none;
  }
  .help {
    font-size: 0.8rem;
    margin: 0;
  }
  .help code {
    font-family: var(--mono);
  }

  .props {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
    align-self: start;
    padding: 1rem 1.1rem 1.2rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-raised);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .label {
    font-size: 0.78rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }
  .row-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }
  .manage {
    font-size: 0.8rem;
  }
  select,
  .field input,
  .add-field input {
    width: 100%;
    padding: 0.35rem 0.55rem;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    font-size: 0.9rem;
  }
  .field > span,
  .field > label {
    font-weight: 600;
  }
  .with-remove {
    display: flex;
    gap: 0.25rem;
  }
  .icon {
    display: grid;
    place-items: center;
    padding: 0.3rem;
    border-color: transparent;
    background: none;
    color: var(--text-muted);
  }
  .add-field {
    display: flex;
    gap: 0.3rem;
  }
  .add-field button {
    padding: 0.3rem 0.6rem;
    font-size: 0.85rem;
  }
  .link {
    align-self: flex-start;
    border: none;
    background: none;
    padding: 0;
    color: var(--accent);
    font-size: 0.9rem;
  }
  .link:hover {
    background: none;
    text-decoration: underline;
  }
  .warn {
    color: var(--danger);
    font-size: 0.8rem;
    margin: 0;
  }
  small {
    font-size: 0.8rem;
  }

  @media (max-width: 900px) {
    .editor {
      grid-template-columns: minmax(0, 1fr);
    }
    .props {
      order: -1;
    }
    .text {
      min-height: 60vh;
    }
  }
</style>
