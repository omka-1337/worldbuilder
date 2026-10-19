<script lang="ts">
  import { onMount, tick } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import ChipsInput from '../components/ChipsInput.svelte'
  import ContextMenu, { type MenuItem } from '../components/ContextMenu.svelte'
  import AnchorDialog from '../components/AnchorDialog.svelte'
  import ImagePicker from '../components/ImagePicker.svelte'
  import { api, key, pageHref, worldHref, HttpError, type FieldDef } from '../lib/api'
  import { navigate, router } from '../lib/router.svelte'
  import { store, refresh } from '../lib/store.svelte'
  import { linkComplete } from '../lib/linkComplete'
  import { exampleDate } from '../lib/calendar'
  import { fieldLabel } from '../lib/fields'
  import { lineChanges } from '../lib/linediff'
  import { plural } from '../lib/format'
  import ChangeGutter from '../components/ChangeGutter.svelte'
  import HighlightLayer from '../components/HighlightLayer.svelte'

  // The article being edited; empty for a brand-new one from "New article".
  let { title }: { title: string } = $props()

  let isNew = $state(false)
  // "Create article" from a folder in the tree: where the new one goes.
  const folder = router.query.get('folder') ?? ''
  let loading = $state(true)
  let saving = $state(false)
  let error = $state('')
  let conflict = $state('') // a title that already belongs to another article

  // The form. Infobox values are kept by field key, so switching the type
  // back and forth does not lose what was typed.
  let name = $state('')
  let type = $state('')
  let values = $state<Record<string, string>>({})
  // Fields of this article only, not of its type, in order. Each can sit in
  // one of the type's sections ('' for none: shown last).
  type Own = { key: string; section: string; value: string }
  let extras = $state<Own[]>([])
  let aliases = $state<string[]>([])
  let categories = $state<string[]>([])
  let body = $state('')
  let image = $state('') // ![[file.png]]; every type has it
  let imageUrl = $state('')
  let caption = $state('')

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
      imageUrl = src?.imageUrl ?? ''
      const known = new Set((store.world?.types[type]?.fields ?? []).map((f) => f.key))
      for (const f of src?.infobox ?? []) {
        if (f.key === 'image') image = f.value
        else if (f.key === 'caption') caption = f.value
        else if (f.section || !known.has(f.key)) extras.push({ key: f.key, section: f.section ?? '', value: f.value })
        else values[f.key] = f.value
      }
      saved = JSON.stringify(build())
      // The text as last saved as a version, to mark what changed since.
      api.committed(src?.exists ? src.title : name || title).then(
        (c) => (base = c.tracked ? c.body : null),
        () => (base = null),
      )
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

  // --- changes and counts ------------------------------------------------

  let base = $state<string | null>(null) // null: no history to compare with
  const changes = $derived(base === null ? null : lineChanges(base, body))
  const chars = $derived([...body.replaceAll('\n', '')].length)
  const words = $derived(body.match(/[\p{L}\p{N}][\p{L}\p{N}'’ʼ-]*/gu)?.length ?? 0)
  const count = (n: number) => n.toLocaleString('uk')

  // --- preview ---------------------------------------------------------------

  let previewing = $state(false)
  let previewHTML = $state('')
  async function showPreview(on: boolean) {
    previewing = on
    if (on) {
      try {
        previewHTML = await api.preview(body)
      } catch (e) {
        previewHTML = `<p class="warn">Не вдалося показати: ${String(e)}</p>`
      }
    } else {
      await tick()
      bodyInput?.focus()
    }
  }

  // --- date anchors: {{date | description}} --------------------------------

  // The right button over the text offers adding a date (or changing the
  // one under the cursor) besides cut, copy and paste; Shift with the right
  // button gives the browser's own menu, with spelling suggestions.
  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  let anchor = $state<{ from: number; to: number; date: string; note: string; editing: boolean } | null>(null)

  function anchorAround(pos: number) {
    for (const m of body.matchAll(/\{\{([^{}\n]*?)\}\}/g)) {
      const from = m.index, to = m.index + m[0].length
      if (from <= pos && pos <= to) {
        const [date, ...note] = m[1].split('|')
        return { from, to, date: date.trim(), note: note.join('|').trim(), editing: true }
      }
    }
    return null
  }

  function onBodyMenu(e: MouseEvent) {
    if (e.shiftKey || !bodyInput) return
    e.preventDefault()
    const { selectionStart: from, selectionEnd: to } = bodyInput
    const selected = body.slice(from, to)
    const existing = anchorAround(from)
    const items: MenuItem[] = [
      existing
        ? { label: 'Змінити дату', icon: 'calendar', run: () => (anchor = existing) }
        : {
            label: 'Додати дату',
            icon: 'calendar',
            run: () => (anchor = { from, to, date: selected.trim(), note: '', editing: false }),
          },
      'separator',
    ]
    if (selected) {
      items.push(
        { label: 'Вирізати', run: () => navigator.clipboard.writeText(selected).then(() => replace(from, to, '')) },
        { label: 'Копіювати', run: () => navigator.clipboard.writeText(selected) },
      )
    }
    items.push({ label: 'Вставити', run: () => navigator.clipboard.readText().then((t) => replace(from, to, t)) })
    menu = { x: e.clientX, y: e.clientY, items }
  }

  // Replaces a span of the text as typing would, so Ctrl+Z undoes it.
  async function replace(from: number, to: number, text: string) {
    if (!bodyInput) return
    bodyInput.focus()
    bodyInput.setSelectionRange(from, to)
    if (!document.execCommand('insertText', false, text)) {
      body = body.slice(0, from) + text + body.slice(to)
      await tick()
      bodyInput.setSelectionRange(from + text.length, from + text.length)
    }
  }

  async function saveAnchor(date: string, note: string) {
    if (!anchor) return
    const { from, to } = anchor
    anchor = null
    await tick() // the dialog is gone: the text can take focus, and the edit joins its undo history
    replace(from, to, note ? `{{${date} | ${note}}}` : `{{${date}}}`)
  }

  function build() {
    const infobox = [
      { key: 'image', value: image },
      { key: 'caption', value: image ? caption : '' },
      ...schema.map((f) => ({ key: f.key, value: values[f.key] ?? '' })),
      ...extras.map((e) => ({ key: e.key, value: e.value, section: e.section || undefined })),
    ]
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
    const now = new Set([...schema.map((f) => f.key), ...extras.filter((e) => !e.section).map((e) => e.key)])
    return (store.world?.types[loadedType]?.fields ?? []).filter((f) => !now.has(f.key) && values[f.key]?.trim())
  })

  async function addField(e: SubmitEvent) {
    e.preventDefault()
    const key = newField.trim()
    if (!key) return
    if (!extras.some((e) => e.key === key && !e.section) && !schema.some((f) => f.key === key))
      extras.push({ key, section: '', value: '' })
    newField = ''
    addingField = false
    await tick()
    document.getElementById(ownId({ key, section: '' }))?.focus()
  }

  // --- own fields in sections -------------------------------------------

  // The type's fields in groups: what comes before the first section, then
  // each section with its fields.
  const groups = $derived.by(() => {
    const out: { key: string; label: string; fields: FieldDef[] }[] = [{ key: '', label: '', fields: [] }]
    for (const f of schema) {
      if (f.type === 'section') out.push({ key: f.key, label: f.label, fields: [] })
      else out.at(-1)!.fields.push(f)
    }
    return out
  })

  // Where own fields can go, top to bottom: the type's sections, sections
  // the type no longer has but some field still sits in, then outside any
  // section, which the article shows last.
  const places = $derived.by(() => {
    const keys = groups.map((g) => g.key).filter(Boolean)
    for (const e of extras) if (e.section && !keys.includes(e.section)) keys.push(e.section)
    return [...keys, '']
  })

  const ownIn = (section: string) => extras.filter((e) => e.section === section)
  const ownId = (e: Pick<Own, 'key' | 'section'>) => `own-${e.section}-${e.key}`

  // Up and down step through the fields of a section and on into the
  // neighbouring one, so a field can be moved anywhere.
  async function moveOwn(e: Own, dir: -1 | 1) {
    const mates = ownIn(e.section)
    const i = mates.indexOf(e)
    const swapWith = mates[i + dir]
    const from = extras.indexOf(e)
    if (swapWith) {
      const to = extras.indexOf(swapWith)
      ;[extras[from], extras[to]] = [extras[to], extras[from]]
    } else {
      const p = places.indexOf(e.section) + dir
      if (p < 0 || p >= places.length || ownIn(places[p]).some((o) => o.key === e.key)) return
      const [moved] = extras.splice(from, 1)
      moved.section = places[p]
      // Last of the section above, or first of the one below.
      if (dir < 0) extras.push(moved)
      else extras.unshift(moved)
    }
    await tick()
    document.getElementById(ownId(e))?.focus()
  }

  const isFirst = (e: Own) => places.indexOf(e.section) === 0 && ownIn(e.section)[0] === e
  const isLast = (e: Own) => e.section === '' && ownIn('').at(-1) === e

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
      const newTitle = await api.saveSource(isNew ? name.trim() : originalName, { ...build(), create: isNew, folder: isNew ? folder : '' })
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
  ondragover={(e) => e.dataTransfer?.types.includes('Files') && e.preventDefault()}
  ondrop={(e) => e.dataTransfer?.types.includes('Files') && e.preventDefault()}
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
      <label for="name" class="label">
        {isNew ? 'Нова стаття' : 'Назва статті'}{#if isNew && folder}<span class="folder">&nbsp;· у папці {folder.replaceAll('/', ' / ')}</span>{/if}
      </label>
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

    <div class="body-area">
    <div class="modes" role="tablist" aria-label="Текст чи перегляд">
      <button role="tab" aria-selected={!previewing} class:on={!previewing} onclick={() => showPreview(false)}>Текст</button>
      <button role="tab" aria-selected={previewing} class:on={previewing} onclick={() => showPreview(true)}>Перегляд</button>
    </div>
    <div class="body-wrap" hidden={previewing}>
      <HighlightLayer textarea={bodyInput} text={body} />
      <textarea
        id="body"
        class="body"
        bind:this={bodyInput}
        bind:value={body}
        use:linkComplete={pages}
        oncontextmenu={onBodyMenu}
        spellcheck="true"
        placeholder="Почніть писати. Наберіть [[, щоб послатися на іншу статтю."
        aria-label="Текст статті"
      ></textarea>
      <ChangeGutter textarea={bodyInput} text={body} {changes} />
    </div>
    {#if previewing}
      <div class="preview prose" aria-label="Перегляд статті">{@html previewHTML}</div>
    {/if}
    </div>

    <div class="stats" aria-live="off">
      <span>{count(chars)} {plural(chars, ['символ', 'символи', 'символів']).replace(/^\S+ /, '')}</span>
      <span>{count(words)} {plural(words, ['слово', 'слова', 'слів']).replace(/^\S+ /, '')}</span>
      {#if changes}
        <span class="diff" title="Відносно останньої збереженої версії">
          {#if changes.addedCount || changes.deletedCount}
            <span class="plus">+{changes.addedCount}</span>
            <span class="minus">−{changes.deletedCount}</span>
            {plural(Math.max(changes.addedCount, changes.deletedCount), ['рядок', 'рядки', 'рядків']).replace(/^\S+ /, '')}
            від останньої версії
          {:else}
            Без змін від останньої версії
          {/if}
        </span>
      {/if}
    </div>

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

      <div class="group">
        <span class="label">Зображення</span>
        <ImagePicker bind:value={image} bind:url={imageUrl} title={name || title} />
        {#if image}
          <input
            class="caption"
            bind:value={caption}
            use:linkComplete={pages}
            autocomplete="off"
            placeholder="Підпис під зображенням"
            aria-label="Підпис під зображенням"
          />
        {/if}
      </div>

      {#if schema.length || extras.length}
        <div class="group">
          <span class="label">Інфобокс</span>
          {#each groups as g (g.key)}
            {#if g.key}<span class="section">{g.label}</span>{/if}
            {#each g.fields as f (f.key)}
              <label class="field">
                <span>{f.label}</span>
                <input
                  id="field-{f.key}"
                  bind:value={values[f.key]}
                  use:linkComplete={pages}
                  autocomplete="off"
                  placeholder={f.type === 'date' ? `Напр. ${exampleDate(store.world)}` : ''}
                />
                {#if f.type === 'date' && values[f.key]?.trim() && !hasYear(values[f.key])}
                  <small class="warn">Без числа року стаття не з'явиться на таймлайні.</small>
                {/if}
              </label>
            {/each}
            {#if g.key}{@render ownFields(g.key)}{/if}
          {/each}
          {#each places.filter((p) => p && !groups.some((g) => g.key === p)) as p (p)}
            <span class="section">{fieldLabel(store.world, type, p)}</span>
            {@render ownFields(p)}
          {/each}
          {#if ownIn('').length}
            {#if places.length > 1}<span class="section loose">Поза розділами</span>{/if}
            {@render ownFields('')}
          {/if}
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
      <code>## Розділ</code> — заголовок розділу, <code>{'{{'}дата | опис{'}}'}</code> — дата для хронології (права кнопка → «Додати дату»), <code>**жирний**</code>, <code>*курсив*</code>. Ctrl+S — зберегти.
    </p>
  </div>
{/if}

{#snippet ownFields(section: string)}
  {#each ownIn(section) as e (e.key)}
    {@const label = fieldLabel(store.world, type, e.key)}
    <div class="field own">
      <label for={ownId(e)}>{label}</label>
      <div class="with-remove">
        <input id={ownId(e)} bind:value={e.value} use:linkComplete={pages} autocomplete="off" />
        <button class="icon" onclick={() => moveOwn(e, -1)} disabled={isFirst(e)} title="Вище" aria-label="Перемістити «{label}» вище">
          <Icon name="up" size={15} />
        </button>
        <button class="icon" onclick={() => moveOwn(e, 1)} disabled={isLast(e)} title="Нижче" aria-label="Перемістити «{label}» нижче">
          <Icon name="down" size={15} />
        </button>
        <button class="icon" onclick={() => extras.splice(extras.indexOf(e), 1)} title="Прибрати" aria-label="Прибрати поле «{label}»">
          <Icon name="trash" size={15} />
        </button>
      </div>
    </div>
  {/each}
{/snippet}

{#if menu}<ContextMenu {...menu} onclose={() => (menu = null)} />{/if}
{#if anchor}
  <AnchorDialog
    date={anchor.date}
    note={anchor.note}
    editing={anchor.editing}
    onsave={saveAnchor}
    oncancel={() => {
      anchor = null
      bodyInput?.focus()
    }}
  />
{/if}

<style>
  /* Name and actions on top; below, the text and the properties panel share
     one row, so the panel starts and ends exactly with the text box. */
  .editor {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 20rem;
    grid-template-rows: auto minmax(0, 1fr) auto auto;
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
  .folder {
    text-transform: none;
    letter-spacing: 0;
    font-weight: 400;
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

  .body-area {
    grid-area: 2 / 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .body-wrap {
    flex: 1;
    position: relative;
    display: grid;
    min-height: 0;
    overflow: hidden; /* the measuring copies inside must not stretch the page */
    border-radius: 6px;
    background: var(--bg-raised);
  }
  .body-wrap[hidden] {
    display: none;
  }
  /* The letters are drawn by the coloured layer under the textarea. */
  .body {
    position: relative;
    z-index: 1;
    color: transparent;
    caret-color: var(--text);
    background: transparent;
  }
  .body::placeholder {
    color: var(--text-muted);
  }
  .body::selection {
    color: transparent;
    background: color-mix(in srgb, var(--accent) 30%, transparent);
  }
  .preview {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 1rem 1.4rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg-raised);
  }
  .modes {
    display: flex;
    gap: 0.2rem;
    margin-bottom: 0.4rem;
  }
  .modes button {
    padding: 0.2rem 0.7rem;
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    background: none;
    color: var(--text-muted);
    font-size: 0.9rem;
  }
  .modes button:hover {
    color: var(--text);
    background: none;
  }
  .modes button.on {
    color: var(--text);
    border-bottom-color: var(--accent);
    font-weight: 600;
  }
  .stats {
    grid-area: 3 / 1;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1.2rem;
    margin-top: -0.35rem;
    font-size: 0.82rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .stats .diff {
    margin-left: auto;
  }
  .plus {
    color: var(--added);
    font-weight: 600;
  }
  .minus {
    color: var(--removed);
    font-weight: 600;
  }
  .body {
    padding: 1rem 1.1rem;
    font: 1rem/1.7 var(--sans);
    resize: none;
  }

  .props {
    grid-area: 2 / 2 / 4;
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
  .caption,
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
  .section {
    margin-top: 0.4rem;
    padding-bottom: 0.15rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.85rem;
    font-weight: 700;
  }
  .with-remove {
    display: flex;
    gap: 0.1rem;
  }
  .with-remove input {
    flex: 1;
    min-width: 0;
  }
  .section.loose {
    font-weight: 600;
    color: var(--text-muted);
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
    grid-area: 4 / 1 / 5 / -1;
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
    .body-area,
    .stats,
    .props,
    .help {
      grid-area: auto;
    }
    .body {
      min-height: 60vh;
    }
    .props {
      overflow: visible;
    }
  }
</style>
