<script lang="ts">
  import { onMount } from 'svelte'
  import { api, NotFoundError, pageHref, worldHref } from '../lib/api'
  import { navigate } from '../lib/router.svelte'
  import { store, refresh } from '../lib/store.svelte'

  let { title }: { title: string } = $props()

  let content = $state('')
  let saved = $state('')
  let isNew = $state(false)
  let loading = $state(true)
  let saving = $state(false)
  let error = $state('')

  const dirty = $derived(content !== saved)
  const types = $derived(Object.entries(store.world?.types ?? {}).sort((a, b) => a[1].label.localeCompare(b[1].label)))

  onMount(async () => {
    document.title = `Редагування: ${title} — ${store.world?.name}`
    try {
      content = saved = (await api.page(title)).raw
    } catch (e) {
      if (!(e instanceof NotFoundError)) error = String(e)
      isNew = true
      content = template('')
    }
    loading = false
  })

  // A new article starts with frontmatter for the chosen type: its infobox
  // fields are listed empty, ready to fill in.
  function template(type: string) {
    const def = store.world?.types[type]
    const lines = ['---']
    if (type) lines.push(`type: ${type}`)
    lines.push('categories: []')
    if (def?.fields.length) {
      lines.push('infobox:')
      for (const f of def.fields) lines.push(`  ${f.key}:`)
    }
    lines.push('---', `**${title}** — `, '')
    return lines.join('\n')
  }

  async function save() {
    if (saving) return
    saving = true
    error = ''
    try {
      const newTitle = await api.save(title, content)
      saved = content
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
    saved = content
    if (isNew) history.back()
    else navigate(pageHref(title))
  }

  function onKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault()
      save()
    }
  }

  // Tab inserts spaces: YAML frontmatter is indentation-sensitive.
  function onTextareaKeydown(e: KeyboardEvent) {
    if (e.key !== 'Tab' || e.shiftKey) return
    e.preventDefault()
    const ta = e.currentTarget as HTMLTextAreaElement
    ta.setRangeText('  ', ta.selectionStart, ta.selectionEnd, 'end')
    content = ta.value
  }
</script>

<svelte:window
  onkeydown={onKeydown}
  onbeforeunload={(e) => {
    if (dirty) e.preventDefault()
  }}
/>

<div class="editor">
  <header>
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

  {#if isNew && types.length}
    <div class="types">
      <span class="muted">Тип статті:</span>
      <button onclick={() => (content = template(''))}>Без типу</button>
      {#each types as [type, def] (type)}
        <button onclick={() => (content = template(type))}>{def.label}</button>
      {/each}
      <a class="manage" href={worldHref(undefined, '/types')}>Керувати типами…</a>
    </div>
  {/if}

  {#if error}<pre class="error">{error}</pre>{/if}

  {#if !loading}
    <!-- svelte-ignore a11y_autofocus -->
    <textarea bind:value={content} onkeydown={onTextareaKeydown} spellcheck="true" autofocus></textarea>
  {/if}

  <p class="help muted">
    <code>[[Стаття]]</code> або <code>[[Стаття|текст]]</code> — посилання,
    <code>[[Стаття#Розділ]]</code> — на розділ, <code>![[файл.png]]</code> — зображення.
    Ctrl+S — зберегти.
  </p>
</div>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
    max-width: 60rem;
    height: calc(100vh - 6rem);
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .page-title {
    font-size: 1.5rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .types {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    align-items: center;
    font-size: 0.9rem;
  }
  .types button {
    padding: 0.15rem 0.65rem;
  }
  .manage {
    font-size: 0.85rem;
    margin-left: 0.3rem;
  }
  textarea {
    flex: 1;
    min-height: 20rem;
    padding: 1rem;
    font: 0.92rem/1.6 var(--mono);
    resize: none;
    tab-size: 2;
  }
  .error {
    color: var(--danger);
    white-space: pre-wrap;
    margin: 0;
    font-size: 0.85rem;
  }
  .help {
    font-size: 0.82rem;
    margin: 0;
  }
  .help code {
    font-family: var(--mono);
  }
</style>
