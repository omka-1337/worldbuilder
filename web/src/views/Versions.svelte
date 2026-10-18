<script lang="ts">
  import { api, pageHref, HttpError, type Version } from '../lib/api'
  import { store } from '../lib/store.svelte'
  import { plural } from '../lib/format'
  import { changes, refreshChanges, changeName, describe } from '../lib/versions.svelte'

  let log = $state<Version[] | null>(null)
  let message = $state('')
  let saving = $state(false)
  let error = $state('')

  document.title = `Версії — ${store.world?.name}`
  refreshChanges()
  loadLog()

  async function loadLog() {
    try {
      log = await api.versions()
    } catch (e) {
      error = String(e)
    }
  }

  const suggested = $derived(describe(changes.list))

  async function save(e: SubmitEvent) {
    e.preventDefault()
    saving = true
    error = ''
    try {
      await api.saveVersion(message.trim() || suggested)
      message = ''
    } catch (e) {
      // Saved from elsewhere in the meantime: nothing left, which is fine.
      if (!(e instanceof HttpError && e.status === 409)) error = e instanceof Error ? e.message : String(e)
    }
    saving = false
    await Promise.all([refreshChanges(), loadLog()])
  }

  const stateLabel = { added: 'Додано', modified: 'Змінено', deleted: 'Видалено' }

  const when = (iso: string) =>
    new Date(iso).toLocaleString(store.world?.language, { dateStyle: 'long', timeStyle: 'short' })
</script>

<h1 class="page-title">Версії</h1>
<p class="muted">
  Збережена версія — це знімок усього світу на певний момент. Зберігайте версію, коли закінчили якусь частину
  роботи.
</p>

<section>
  <h2>Незбережені зміни</h2>
  {#if !changes.ready}
    <p class="muted">Завантаження…</p>
  {:else if changes.list.length === 0}
    <p class="muted">Усі зміни збережено.</p>
  {:else}
    <ul class="changes">
      {#each changes.list as c (c.path)}
        <li>
          <span class="state {c.state}">{stateLabel[c.state]}</span>
          {#if c.title}<a href={pageHref(c.title)}>{changeName(c)}</a>{:else}<span>{changeName(c)}</span>{/if}
          {#if !c.title && c.path !== 'world.yaml'}<span class="path muted">{c.path}</span>{/if}
        </li>
      {/each}
    </ul>
    <form class="save" onsubmit={save}>
      <input bind:value={message} placeholder={suggested} aria-label="Опис версії" maxlength="500" />
      <button class="primary" disabled={saving}>Зберегти версію</button>
    </form>
    <p class="hint muted">
      Опис можна не писати: тоді буде «{suggested}».
    </p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<section>
  <h2>Історія {#if log}<span class="muted count">({plural(log.length, ['версія', 'версії', 'версій'])})</span>{/if}</h2>
  {#if log?.length === 0}
    <p class="muted">Ще немає жодної версії.</p>
  {:else if log}
    <ol class="log">
      {#each log as v (v.hash)}
        <li>
          <div class="message">{v.message}</div>
          <div class="meta muted">
            {when(v.when)} · {v.author} · <code title={v.hash}>{v.hash.slice(0, 7)}</code>
          </div>
        </li>
      {/each}
    </ol>
  {/if}
</section>

<style>
  section {
    max-width: 52rem;
    margin-top: 1.8rem;
  }
  h2 {
    font-size: 1.15rem;
    margin: 0 0 0.6rem;
  }
  .count {
    font-weight: 400;
    font-size: 0.9rem;
  }
  .changes {
    list-style: none;
    margin: 0 0 1rem;
    padding: 0;
  }
  .changes li {
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
    padding: 0.25rem 0;
  }
  .state {
    flex: none;
    width: 5.5rem;
    font-size: 0.78rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .state.added {
    color: #2f9e58;
  }
  .state.modified {
    color: var(--accent);
  }
  .state.deleted {
    color: var(--danger);
  }
  .path {
    font-size: 0.82rem;
    overflow-wrap: anywhere;
  }
  .save {
    display: flex;
    gap: 0.6rem;
  }
  .save input {
    flex: 1;
    min-width: 0;
  }
  .hint {
    font-size: 0.85rem;
    margin: 0.4rem 0 0;
  }
  .error {
    color: var(--danger);
  }
  .log {
    list-style: none;
    margin: 0;
    padding: 0;
    border-left: 2px solid var(--border);
  }
  .log li {
    position: relative;
    padding: 0.35rem 0 0.7rem 1.1rem;
  }
  .log li::before {
    content: '';
    position: absolute;
    left: -6px;
    top: 0.65rem;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--bg);
    border: 2px solid var(--accent);
  }
  .message {
    font-weight: 600;
  }
  .meta {
    font-size: 0.82rem;
  }
  .meta code {
    font-family: var(--mono);
  }
</style>
