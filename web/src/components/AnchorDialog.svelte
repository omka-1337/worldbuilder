<script lang="ts">
  import { tick } from 'svelte'
  import { exampleDate } from '../lib/calendar'
  import { store } from '../lib/store.svelte'

  // Asks for a date anchor's date and description: {{date | description}}.
  let {
    date = '',
    note = '',
    editing = false,
    onsave,
    oncancel,
  }: {
    date?: string
    note?: string
    editing?: boolean // changing an anchor already in the text
    onsave: (date: string, note: string) => void
    oncancel: () => void
  } = $props()

  let el = $state<HTMLDialogElement>()
  // svelte-ignore state_referenced_locally
  let dateValue = $state(date)
  // svelte-ignore state_referenced_locally
  let noteValue = $state(note)
  let dateInput = $state<HTMLInputElement>()
  let noteInput = $state<HTMLInputElement>()

  $effect(() => {
    if (!el) return
    el.showModal()
    // The date is often the selected text already: go on to the description.
    tick().then(() => (dateValue.trim() ? noteInput : dateInput)?.focus())
  })

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (dateValue.trim()) onsave(dateValue.trim(), noteValue.trim())
  }
</script>

<dialog bind:this={el} onclose={oncancel} onclick={(e) => e.target === el && oncancel()}>
  <form onsubmit={submit}>
    <h2>{editing ? 'Змінити дату' : 'Додати дату'}</h2>
    <p>Дата з’явиться в тексті, а опис — у хронології статті.</p>
    <label>
      <span>Дата</span>
      <input bind:this={dateInput} bind:value={dateValue} placeholder="Напр. {exampleDate(store.world)}" autocomplete="off" />
    </label>
    <label>
      <span>Опис</span>
      <input bind:this={noteInput} bind:value={noteValue} placeholder="Що сталося" autocomplete="off" />
    </label>
    <div class="actions">
      <button type="button" onclick={oncancel}>Скасувати</button>
      <button class="primary" disabled={!dateValue.trim()}>{editing ? 'Зберегти' : 'Додати'}</button>
    </div>
  </form>
</dialog>

<style>
  dialog {
    width: min(28rem, calc(100vw - 2rem));
    padding: 1.1rem 1.25rem 1.2rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-raised);
    color: var(--text);
    box-shadow: 0 20px 50px rgb(0 0 0 / 0.25);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.3);
  }
  h2 {
    margin: 0 0 0.4rem;
    font-size: 1.05rem;
  }
  p {
    margin: 0 0 0.9rem;
    font-size: 0.9rem;
    color: var(--text-muted);
  }
  label {
    display: grid;
    gap: 0.25rem;
    margin-bottom: 0.7rem;
    font-size: 0.85rem;
    font-weight: 600;
  }
  input {
    width: 100%;
    padding: 0.45rem 0.65rem;
    font-weight: 400;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1rem;
  }
</style>
