<script lang="ts">
  import { tick } from 'svelte'
  import Icon from './Icon.svelte'
  import { key, worldHref, type TreeNode } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { router, titleFrom } from '../lib/router.svelte'
  import { prefs } from '../lib/prefs.svelte'
  import { chat, loadChat, send, stop, newChat, type Part, type Mention } from '../lib/agent.svelte'

  loadChat()

  let input = $state('')
  let list = $state<HTMLElement>()
  let field = $state<HTMLTextAreaElement>()

  // The article open in the app goes with each message, so "this article"
  // means something to the assistant.
  const article = $derived.by(() => {
    for (const sub of ['/wiki/', '/edit/']) {
      const prefix = new URL(worldHref(undefined, sub), location.href).pathname
      if (router.path.startsWith(prefix) && router.path.length > prefix.length) return titleFrom(router.path.slice(prefix.length))
    }
    return ''
  })

  // Follow the answer as it grows.
  $effect(() => {
    void chat.messages.length
    void chat.messages.at(-1)?.parts.length
    void (chat.messages.at(-1)?.parts.at(-1) as { text?: string } | undefined)?.text
    tick().then(() => list?.scrollTo({ top: list.scrollHeight }))
  })

  function submit() {
    const text = input.trim()
    if (!text || chat.running) return
    const pointed = present // before the field empties: they are read from it
    input = ''
    mentions = []
    send(text, article || undefined, pointed)
  }

  // --- @ mentions: point the assistant at articles and folders ------------

  let mentions = $state<Mention[]>([])
  // Only those still written in the message count.
  const present = $derived(mentions.filter((m) => input.includes('@' + m.label)))

  // Everything that can be pointed at: articles by title, folders by path.
  const targets = $derived.by(() => {
    const out: (Mention & { hint?: string; aliases: string[] })[] = store.pages.map((p) => ({
      label: p.title,
      title: p.title,
      aliases: p.aliases,
    }))
    const walk = (nodes: TreeNode[]) => {
      for (const n of nodes) {
        if (!n.title) out.push({ label: n.path + '/', folder: n.path, aliases: [] })
        if (n.children) walk(n.children)
      }
    }
    walk(store.tree)
    return out
  })

  let query = $state<{ at: number; text: string } | null>(null)
  let active = $state(0)

  // The @ being typed, from the cursor back: after a space or at the start,
  // within the line.
  function findQuery() {
    if (!field) return (query = null)
    const caret = field.selectionStart
    const before = input.slice(0, caret)
    const at = before.lastIndexOf('@')
    if (at < 0 || (at > 0 && !/\s/.test(before[at - 1]))) return (query = null)
    const text = before.slice(at + 1)
    query = text.includes('\n') || text.length > 60 ? null : { at, text }
    active = 0
  }

  const suggestions = $derived.by(() => {
    if (!query) return []
    const q = key(query.text)
    const scored = targets
      .map((t) => {
        const names = [t.label, ...t.aliases].map(key)
        const starts = names.some((n) => n.startsWith(q))
        const has = starts || names.some((n) => n.includes(q))
        return { t, rank: !has ? -1 : starts ? 0 : 1 }
      })
      .filter((s) => s.rank >= 0)
    scored.sort((a, b) => a.rank - b.rank || Number(!!a.t.folder) - Number(!!b.t.folder) || a.t.label.length - b.t.label.length)
    return scored.slice(0, 8).map((s) => s.t)
  })

  async function choose(t: Mention) {
    if (!query || !field) return
    const end = field.selectionStart
    const insert = '@' + t.label + ' '
    input = input.slice(0, query.at) + insert + input.slice(end)
    if (!mentions.some((m) => m.label === t.label)) mentions.push({ label: t.label, title: t.title, folder: t.folder })
    const caret = query.at + insert.length
    query = null
    await tick()
    field.focus()
    field.setSelectionRange(caret, caret)
  }

  function onFieldKeydown(e: KeyboardEvent) {
    if (query && suggestions.length) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        active = (active + (e.key === 'ArrowDown' ? 1 : suggestions.length - 1)) % suggestions.length
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        choose(suggestions[active])
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        query = null
        return
      }
    }
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      submit()
    }
  }

  const verbs: Record<string, string> = {
    Read: 'Читає',
    Edit: 'Редагує',
    MultiEdit: 'Редагує',
    Write: 'Пише',
    Glob: 'Шукає файли',
    Grep: 'Шукає в текстах',
  }
  const icons: Record<string, 'file' | 'pencil' | 'list'> = { Read: 'file', Edit: 'pencil', MultiEdit: 'pencil', Write: 'pencil', Glob: 'list', Grep: 'list' }
  // A file reads as the article it is; a search, as what it looks for.
  const named = (p: Extract<Part, { kind: 'tool' }>) =>
    /\.md$/i.test(p.target) ? (p.target.split('/').at(-1) ?? p.target).slice(0, -3) : p.target

  const examples = [
    'Що в статтях сказано про Тандор?',
    'Знайди суперечності в датах між статтями',
    'Додай у цю статтю якорі дат для ключових подій',
  ]
</script>

<aside class="agent" aria-label="Агент">
  <header>
    <Icon name="sparkles" size={17} />
    <h2>Агент</h2>
    <button class="icon" onclick={newChat} title="Новий чат" aria-label="Новий чат" disabled={!chat.messages.length && !chat.running}>
      <Icon name="plus" size={16} />
    </button>
    <button class="icon" onclick={() => (prefs.agentOpen = false)} title="Закрити" aria-label="Закрити агента">
      <Icon name="close" size={16} />
    </button>
  </header>

  {#if chat.status && !chat.status.available}
    <div class="empty">
      <p><strong>Не знайдено Claude Code.</strong></p>
      <p>
        Агент працює через Claude Code, встановлений на цьому комп’ютері, з вашим обліковим записом Claude. Встановіть
        його, увійдіть (команда <code>claude</code> у терміналі), а потім перезапустіть Worldbuilder.
      </p>
    </div>
  {:else}
    <div class="messages" bind:this={list}>
      {#if !chat.messages.length}
        <div class="empty">
          <p>Запитайте про свій світ або попросіть щось дописати чи виправити. Агент читає статті й може їх редагувати; кожну зміну видно у «Версіях».</p>
          <div class="examples">
            {#each examples as ex (ex)}
              <button onclick={() => ((input = ex), field?.focus())}>{ex}</button>
            {/each}
          </div>
          {#if chat.status?.rulesFile}
            <p class="muted small">
              Власні правила для агента в цьому світі можна записати у файл <code>{chat.status.rulesFile}</code> у папці
              світу.
            </p>
          {/if}
        </div>
      {/if}
      {#each chat.messages as m, i (i)}
        <div class="msg {m.role}">
          {#if m.role === 'user'}
            {#if m.article}<div class="context muted">Стаття: {m.article}</div>{/if}
            <div class="bubble">{(m.parts[0] as { text: string }).text}</div>
            {#if m.mentions?.length}
              <div class="chips sent">
                {#each m.mentions as t (t.label)}
                  <span class="chip"><Icon name={t.folder ? 'folder' : 'file'} size={12} />{t.folder ? t.label : t.title}</span>
                {/each}
              </div>
            {/if}
          {:else}
            {#each m.parts as p, j (j)}
              {#if p.kind === 'tool'}
                <div class="step muted">
                  <Icon name={icons[p.tool] ?? 'file'} size={13} />
                  <span>{verbs[p.tool] ?? p.tool}</span>
                  {#if p.target}<span class="target">{named(p)}</span>{/if}
                </div>
              {:else if p.html}
                <div class="answer prose">{@html p.html}</div>
              {:else}
                <div class="answer plain">{p.text}</div>
              {/if}
            {/each}
            {#if chat.running && i === chat.messages.length - 1}
              <div class="thinking muted"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div>
            {/if}
            {#if m.error === 'stopped'}
              <p class="note muted">Зупинено.</p>
            {:else if m.error === 'limit'}
              <p class="note error">Ліміт Claude на зараз вичерпано. Спробуйте пізніше.</p>
            {:else if m.error}
              <p class="note error">Помилка: {m.error}</p>
            {/if}
          {/if}
        </div>
      {/each}
    </div>

    <form
      class="compose"
      onsubmit={(e) => {
        e.preventDefault()
        submit()
      }}
    >
      {#if article}<div class="context muted">Стаття: {article}</div>{/if}
      {#if present.length}
        <div class="chips">
          {#each present as t (t.label)}
            <span class="chip"><Icon name={t.folder ? 'folder' : 'file'} size={12} />{t.folder ? t.label : t.title}</span>
          {/each}
        </div>
      {/if}
      {#if query && suggestions.length}
        <ul class="mentions" role="listbox" aria-label="Статті й папки">
          {#each suggestions as t, i (t.label)}
            <li role="option" aria-selected={i === active}>
              <button type="button" class:on={i === active} onmousedown={(e) => (e.preventDefault(), choose(t))}>
                <Icon name={t.folder ? 'folder' : 'file'} size={14} />
                <span class="name">{t.folder ? t.label : t.title}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
      <textarea
        bind:this={field}
        bind:value={input}
        rows="3"
        placeholder="Повідомлення агенту…  @ — вказати статтю чи папку"
        aria-label="Повідомлення агенту"
        oninput={findQuery}
        onclick={findQuery}
        onkeyup={(e) => (e.key === 'ArrowLeft' || e.key === 'ArrowRight') && findQuery()}
        onblur={() => (query = null)}
        onkeydown={onFieldKeydown}
      ></textarea>
      <div class="row">
        {#if chat.status?.version}<span class="muted small">{chat.status.version}</span>{/if}
        {#if chat.running}
          <button type="button" onclick={stop}>Зупинити</button>
        {:else}
          <button class="primary" disabled={!input.trim()}>Надіслати</button>
        {/if}
      </div>
    </form>
  {/if}
</aside>

<style>
  .agent {
    position: sticky;
    top: 0;
    height: 100vh;
    width: 26rem;
    flex: none;
    display: flex;
    flex-direction: column;
    border-left: 1px solid var(--border);
    background: var(--bg-sidebar);
  }
  header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.7rem 0.6rem 0.7rem 0.9rem;
    border-bottom: 1px solid var(--border);
    color: var(--accent);
  }
  h2 {
    flex: 1;
    margin: 0;
    font-size: 0.95rem;
    color: var(--text);
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
    background: var(--bg-hover);
  }
  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 0.8rem 0.9rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .empty {
    padding: 0.4rem 0.9rem;
    font-size: 0.9rem;
  }
  .examples {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin: 0.8rem 0;
  }
  .examples button {
    text-align: left;
    font-size: 0.85rem;
    color: var(--text-muted);
    background: var(--bg-raised);
  }
  .examples button:hover {
    color: var(--text);
  }
  .small {
    font-size: 0.8rem;
  }
  .msg.user {
    align-self: flex-end;
    max-width: 88%;
  }
  .bubble {
    padding: 0.5rem 0.75rem;
    border-radius: 12px 12px 3px 12px;
    background: var(--accent-soft);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-size: 0.92rem;
  }
  .context {
    font-size: 0.75rem;
    margin-bottom: 0.2rem;
    text-align: right;
  }
  .msg.assistant {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    font-size: 0.92rem;
  }
  .step {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.8rem;
  }
  .target {
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .answer.plain {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .answer.prose {
    font-size: 0.92rem;
    line-height: 1.55;
  }
  .answer.prose :global(> :first-child) {
    margin-top: 0;
  }
  .answer.prose :global(> :last-child) {
    margin-bottom: 0;
  }
  .answer.prose :global(h1),
  .answer.prose :global(h2),
  .answer.prose :global(h3) {
    font-size: 1rem;
    margin: 0.8rem 0 0.3rem;
    border: none;
  }
  .note {
    margin: 0;
    font-size: 0.85rem;
  }
  .error {
    color: var(--danger);
  }
  .thinking {
    display: flex;
    gap: 0.25rem;
    padding: 0.3rem 0;
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    animation: blink 1.2s infinite ease-in-out;
  }
  .dot:nth-child(2) {
    animation-delay: 0.2s;
  }
  .dot:nth-child(3) {
    animation-delay: 0.4s;
  }
  @keyframes blink {
    0%,
    80%,
    100% {
      opacity: 0.25;
    }
    40% {
      opacity: 1;
    }
  }
  .compose {
    position: relative;
    padding: 0.6rem 0.8rem 0.8rem;
    border-top: 1px solid var(--border);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    margin-bottom: 0.35rem;
  }
  .chips.sent {
    justify-content: flex-end;
    margin: 0.3rem 0 0;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.1rem 0.45rem;
    border-radius: 999px;
    background: var(--accent-soft);
    color: var(--accent);
    font-size: 0.78rem;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Suggestions for @, above the field. */
  .mentions {
    position: absolute;
    left: 0.8rem;
    right: 0.8rem;
    bottom: calc(100% - 0.2rem);
    z-index: 5;
    list-style: none;
    margin: 0;
    padding: 0.25rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.18);
  }
  .mentions button {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    width: 100%;
    padding: 0.3rem 0.5rem;
    border: none;
    border-radius: 5px;
    background: none;
    color: var(--text);
    text-align: left;
    font-size: 0.88rem;
  }
  .mentions button.on {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .mentions .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .compose .context {
    text-align: left;
  }
  textarea {
    width: 100%;
    resize: none;
    padding: 0.5rem 0.65rem;
    font: inherit;
    font-size: 0.92rem;
    line-height: 1.45;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 0.4rem;
  }
  .row .small {
    margin-right: auto;
  }
  @media (max-width: 800px) {
    .agent {
      position: fixed;
      inset: 0;
      width: auto;
      height: auto;
      z-index: 30;
    }
  }
</style>
