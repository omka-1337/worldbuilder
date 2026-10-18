<script lang="ts">
  import { onMount, tick } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import ChipsInput from '../components/ChipsInput.svelte'
  import { api, key, pageHref, worldHref, HttpError, type FieldDef } from '../lib/api'
  import { navigate } from '../lib/router.svelte'
  import { store, refresh } from '../lib/store.svelte'
  import { linkComplete } from '../lib/linkComplete'

  // The article being edited; empty for a brand-new one from "New article".
  let { title }: { title: string } = $props()

  let isNew = $state(false)
  let loading = $state(true)
  let saving = $state(false)
  let error = $state('')
  let conflict = $state('') // a title that already belongs to another article

  // The form. Infobox values are kept by field key, so switching the type
  // back and forth does not lose what was typed.
  let name = $state('')
  let type = $state('')
  let values = $state<Record<string, string>>({})
  let extras = $state<string[]>([]) // fields of this article only, not of its type
  let aliases = $state<string[]>([])
  let categories = $state<string[]>([])
  let body = $state('')

  let loadedType = ''
  let originalName = $state('')
  let saved = $state('')
  let allCategories = $state<string[]>([])
  let newField = $state('')
  let addingField = $state(false)
  let newFieldInput = $state<HTMLInputElement>()
  let nameInput = $state<HTMLInputElement>()
  let bodyInput = $state<HTMLTextAreaElement>()

  const types = $derived(Object.entries(store.world?.types ?? {}).sort((a, b) => a[1].label.localeCompare(b[1].label)))
  const schema = $derived<FieldDef[]>(store.world?.types[type]?.fields ?? [])
  const era = $derived(store.world?.era ? ' ' + store.world.era : '')
  const pages = () => store.pages

  onMount(async () => {
    api.categories().then((c) => (allCategories = c.map((x) => x.name)))
    try {
      const src = title ? await api.source(title) : null
      isNew = !src?.exists
      name = originalName = src?.title ?? ''
      type = loadedType = src?.type ?? ''
      aliases = src?.aliases ?? []
      categories = src?.categories ?? []
      body = src?.body ?? ''
      const known = new Set((store.world?.types[type]?.fields ?? []).map((f) => f.key))
      for (const f of src?.infobox ?? []) {
        values[f.key] = f.value
        if (!known.has(f.key)) extras.push(f.key)
      }
      saved = JSON.stringify(build())
    } catch (e) {
      error = String(e)
    }
    loading = false
    await tick()
    // A new article starts at its name unless it came with one (a red link).
    ;(isNew && !name ? nameInput : bodyInput)?.focus()
  })

  $effect(() => {
    document.title = `${isNew ? 'Нова стаття' : 'Редагування'}${name ? ': ' + name : ''} — ${store.world?.name}`
  })

  function build() {
    const infobox = [...schema.map((f) => f.key), ...extras].map((key) => ({ key, value: values[key] ?? '' }))
    return { title: name.trim(), type, aliases, categories, infobox, body }
  }

  const dirty = $derived(!loading && JSON.stringify(build()) !== saved)
  const renaming = $derived(!isNew && !!name.trim() && key(name) !== key(originalName))
  // Warn while typing, before the server would refuse it.
  const taken = $derived.by(() => {
    if (!name.trim() || (!isNew && !renaming)) return ''
    const k = key(name)
    const hit = store.pages.find((p) => key(p.title) === k || p.aliases.some((a) => key(a) === k))
    return hit && key(hit.title) !== key(originalName) ? hit.title : ''
  })

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
    if (!name.trim()) {
      error = 'Дайте статті назву.'
      nameInput?.focus()
      return
    }
    saving = true
    error = conflict = ''
    try {
      const newTitle = await api.saveSource(isNew ? name.trim() : originalName, { ...build(), create: isNew })
      saved = JSON.stringify(build())
      await refresh()
      navigate(pageHref(newTitle), { replace: true })
    } catch (e) {
      if (e instanceof HttpError && e.status === 409) conflict = name.trim()
      else error = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  function cancel() {
    if (dirty && !confirm('Відкинути незбережені зміни?')) return
    saved = JSON.stringify(build())
    if (isNew) history.back()
    else navigate(pageHref(originalName))
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

{#if !loading}
  <div class="editor">
    <div class="name">
      <label for="name" class="label">{isNew ? 'Нова стаття' : 'Назва статті'}</label>
      <input
        id="name"
        bind:this={nameInput}
        bind:value={name}
        placeholder="Назва статті"
        autocomplete="off"
        onkeydown={(e) => e.key === 'Enter' && bodyInput?.focus()}
      />
      {#if taken || conflict}
        <p class="warn">
          Стаття <a href={pageHref(taken || conflict)}>«{taken || conflict}»</a> вже існує.
          {isNew ? 'Оберіть іншу назву або відкрийте її.' : 'Оберіть іншу назву.'}
        </p>
      {:else if renaming}
        <p class="note">
          Статтю буде перейменовано. Стара назва «{originalName}» стане іншою назвою, тож посилання на неї працюватимуть.
        </p>
      {/if}
      {#if error}<p class="warn">{error}</p>{/if}
    </div>

    <div class="actions">
      <button onclick={cancel}>Скасувати</button>
      <button class="primary" onclick={save} disabled={saving || !!taken}>
        {saving ? 'Збереження…' : 'Зберегти'}
      </button>
    </div>

    <textarea
      id="body"
      class="body"
      bind:this={bodyInput}
      bind:value={body}
      use:linkComplete={pages}
      spellcheck="true"
      placeholder="Почніть писати. Наберіть [[, щоб послатися на іншу статтю."
      aria-label="Текст статті"
    ></textarea>

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
                use:linkComplete={pages}
                autocomplete="off"
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
                <input id="field-{key}" bind:value={values[key]} use:linkComplete={pages} autocomplete="off" />
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

    <p class="help muted">
      <code>[[</code> — посилання на статтю (з підказками), <code>[[Стаття|текст]]</code> — посилання з іншим текстом,
      <code>## Розділ</code> — заголовок розділу, <code>**жирний**</code>, <code>*курсив*</code>. Ctrl+S — зберегти.
    </p>
  </div>
{/if}

<style>
  /* Name and actions on top; below, the text and the properties panel share
     one row, so the panel starts and ends exactly with the text box. */
  .editor {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 20rem;
    grid-template-rows: auto minmax(0, 1fr) auto;
    gap: 0.75rem 1.5rem;
    max-width: 76rem;
    height: calc(100vh - 6rem);
    min-height: 32rem;
  }
  .name {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .name input {
    font-size: 1.6rem;
    font-weight: 650;
    padding: 0.35rem 0.7rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
    align-self: end;
    padding-bottom: 0.35rem;
  }
  .note,
  .warn {
    margin: 0;
    font-size: 0.82rem;
  }
  .note {
    color: var(--text-muted);
  }
  .warn {
    color: var(--danger);
  }

  .body {
    padding: 1rem 1.1rem;
    font: 1rem/1.7 var(--sans);
    resize: none;
  }

  .props {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
    overflow-y: auto;
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
  small {
    font-size: 0.8rem;
  }
  .help {
    grid-column: 1 / -1;
    font-size: 0.8rem;
    margin: 0;
  }
  .help code {
    font-family: var(--mono);
  }

  @media (max-width: 900px) {
    .editor {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: none;
      height: auto;
    }
    .actions {
      justify-content: flex-start;
    }
    .body {
      min-height: 60vh;
    }
    .props {
      overflow: visible;
    }
  }
</style>
