<script lang="ts">
  import Icon from './Icon.svelte'
  import { api } from '../lib/api'
  import { prefs, type Side, type Theme } from '../lib/prefs.svelte'

  let { open = $bindable(false) }: { open?: boolean } = $props()

  let dialog: HTMLDialogElement
  let root = $state('')

  $effect(() => {
    if (open && !dialog.open) {
      dialog.showModal()
      api.worlds().then((r) => (root = r.root))
    } else if (!open && dialog.open) dialog.close()
  })

  const themes: [Theme, string][] = [
    ['system', 'Як у системі'],
    ['light', 'Світла'],
    ['dark', 'Темна'],
  ]
  const sides: [Side, string][] = [
    ['left', 'Ліворуч'],
    ['right', 'Праворуч'],
  ]
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} onclick={(e) => e.target === dialog && dialog.close()}>
  <header>
    <h2>Налаштування</h2>
    <button class="close" onclick={() => dialog.close()} aria-label="Закрити"><Icon name="close" size={18} /></button>
  </header>

  <fieldset>
    <legend>Тема</legend>
    <div class="segmented">
      {#each themes as [value, label] (value)}
        <label class:on={prefs.theme === value}>
          <input type="radio" name="theme" {value} bind:group={prefs.theme} />{label}
        </label>
      {/each}
    </div>
  </fieldset>

  <fieldset>
    <legend>Панель інструментів</legend>
    <div class="segmented">
      {#each sides as [value, label] (value)}
        <label class:on={prefs.ribbon === value}>
          <input type="radio" name="ribbon" {value} bind:group={prefs.ribbon} />{label}
        </label>
      {/each}
    </div>
    <p class="muted hint">Панель також можна перетягнути за верхній край.</p>
  </fieldset>

  <fieldset>
    <legend>Бібліотека світів</legend>
    <code>{root || '…'}</code>
    <p class="muted hint">Змінюється параметром запуску <code>-library</code>.</p>
  </fieldset>
</dialog>

<style>
  dialog {
    width: min(28rem, calc(100vw - 2rem));
    padding: 1.25rem 1.4rem 1.4rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-raised);
    color: var(--text);
    box-shadow: 0 20px 50px rgb(0 0 0 / 0.25);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.35);
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
  }
  h2 {
    margin: 0;
    font-size: 1.15rem;
  }
  .close {
    border: none;
    background: none;
    padding: 0.25rem;
    display: grid;
    color: var(--text-muted);
  }
  fieldset {
    border: none;
    padding: 0;
    margin: 1rem 0 0;
  }
  legend {
    font-weight: 600;
    font-size: 0.9rem;
    margin-bottom: 0.4rem;
  }
  .segmented {
    display: inline-flex;
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .segmented label {
    padding: 0.3rem 0.8rem;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .segmented label + label {
    border-left: 1px solid var(--border);
  }
  .segmented label.on {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .segmented input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .segmented label:has(input:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  code {
    font-family: var(--mono);
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }
  .hint {
    font-size: 0.8rem;
    margin: 0.35rem 0 0;
  }
</style>
