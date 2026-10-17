<script lang="ts">
  import { api, worldHref, type WorldInfo } from '../lib/api'
  import { navigate } from '../lib/router.svelte'
  import { ago, plural } from '../lib/format'

  let root = $state('')
  let worlds = $state<WorldInfo[] | null>(null)
  let error = $state('')
  let name = $state('')
  let creating = $state(false)

  document.title = 'Worldbuilder'
  api
    .worlds()
    .then((r) => ({ root, worlds } = r))
    .catch((e) => (error = String(e)))

  async function create(e: SubmitEvent) {
    e.preventDefault()
    if (!name.trim() || creating) return
    creating = true
    error = ''
    try {
      const info = await api.createWorld(name.trim())
      navigate(worldHref(info.id))
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      creating = false
    }
  }
</script>

<main class="standalone">
  <header>
    <h1 class="page-title">Worldbuilder</h1>
    <p class="muted">Оберіть світ, над яким працюватимете, або створіть новий.</p>
  </header>

  {#if worlds}
    <div class="grid">
      {#each worlds as w (w.id)}
        <a class="card world" href={worldHref(w.id)}>
          <span class="name">{w.name}</span>
          <span class="desc" class:muted={!w.description}>{w.description || 'Без опису'}</span>
          <span class="meta muted">
            {plural(w.pages, ['стаття', 'статті', 'статей'])} · змінено {ago(w.modified)}
          </span>
        </a>
      {/each}

      <form class="card create" onsubmit={create}>
        <span class="name">{worlds.length ? 'Новий світ' : 'Створіть перший світ'}</span>
        <input bind:value={name} placeholder="Назва світу" aria-label="Назва світу" />
        <button class="primary" disabled={!name.trim() || creating}>
          {creating ? 'Створення…' : 'Створити'}
        </button>
      </form>
    </div>
  {/if}

  {#if error}<p class="error">{error}</p>{/if}

  {#if root}
    <p class="where muted">
      Світи зберігаються в <code>{root}</code>. Кожен світ — окрема тека з Markdown-файлами,
      її можна відкривати й редагувати будь-яким редактором або синхронізувати через git чи хмару.
    </p>
  {/if}
</main>

<style>
  header {
    margin-bottom: 2rem;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
    gap: 1rem;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    padding: 1.1rem 1.2rem;
    min-height: 9rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-raised);
  }
  .world {
    color: var(--text);
    transition: border-color 0.15s, transform 0.15s;
  }
  .world:hover {
    text-decoration: none;
    border-color: var(--accent);
    transform: translateY(-1px);
  }
  .name {
    font-weight: 650;
    font-size: 1.1rem;
    overflow-wrap: anywhere;
  }
  .desc {
    flex: 1;
    font-size: 0.92rem;
  }
  .meta {
    font-size: 0.82rem;
  }
  .create {
    border-style: dashed;
    background: transparent;
  }
  .create input {
    padding: 0.4rem 0.65rem;
  }
  .create button {
    align-self: flex-start;
  }
  .error {
    color: var(--danger);
  }
  .where {
    margin-top: 2.5rem;
    font-size: 0.85rem;
  }
  .where code {
    font-family: var(--mono);
    overflow-wrap: anywhere;
  }
</style>
