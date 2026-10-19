<script lang="ts">
  import { tick } from 'svelte'
  import Icon from './Icon.svelte'
  import { worldHref } from '../lib/api'
  import { router, titleFrom } from '../lib/router.svelte'
  import { prefs } from '../lib/prefs.svelte'
  import { chat, loadChat, send, stop, newChat, type Part } from '../lib/agent.svelte'

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
    input = ''
    send(text, article || undefined)
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
      <textarea
        bind:this={field}
        bind:value={input}
        rows="3"
        placeholder="Повідомлення агенту…  (Enter — надіслати, Shift+Enter — новий рядок)"
        aria-label="Повідомлення агенту"
        onkeydown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
            e.preventDefault()
            submit()
          }
        }}
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
    padding: 0.6rem 0.8rem 0.8rem;
    border-top: 1px solid var(--border);
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
