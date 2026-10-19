<script lang="ts">
  import { tick } from 'svelte'
  import { store } from '../lib/store.svelte'

  // Asks for a link's article and, optionally, the text shown for it:
  // [[article|text]].
  let {
    target = '',
    label = '',
    editing = false,
    onsave,
    oncancel,
  }: {
    target?: string
    label?: string
    editing?: boolean // changing a link already in the text
    onsave: (target: string, label: string) => void
    oncancel: () => void
  } = $props()

  let el = $state<HTMLDialogElement>()
  // svelte-ignore state_referenced_locally
  let targetValue = $state(target)
  // svelte-ignore state_referenced_locally
  let labelValue = $state(label)
  let targetInput = $state<HTMLInputElement>()
  let labelInput = $state<HTMLInputElement>()

  // Every title and other name, for the browser to suggest as one types.
  const names = $derived([...new Set(store.pages.flatMap((p) => [p.title, ...p.aliases]))])

  $effect(() => {
    if (!el) return
    el.showModal()
    tick().then(() => (targetValue.trim() ? labelInput : targetInput)?.focus())
  })

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (targetValue.trim()) onsave(targetValue.trim(), labelValue.trim())
  }
</script>

<dialog bind:this={el} onclose={oncancel} onclick={(e) => e.target === el && oncancel()}>
  <form onsubmit={submit}>
    <h2>{editing ? 'Змінити посилання' : 'Додати посилання'}</h2>
    <label>
      <span>Стаття</span>
      <input bind:this={targetInput} bind:value={targetValue} list="link-targets" placeholder="Назва статті" autocomplete="off" />
    </label>
    <label>
      <span>Текст посилання</span>
      <input bind:this={labelInput} bind:value={labelValue} placeholder={targetValue.trim() || 'Як у назві статті'} autocomplete="off" />
    </label>
    <datalist id="link-targets">
      {#each names as n (n)}<option value={n}></option>{/each}
    </datalist>
    <div class="actions">
      <button type="button" onclick={oncancel}>Скасувати</button>
      <button class="primary" disabled={!targetValue.trim()}>{editing ? 'Зберегти' : 'Додати'}</button>
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
    margin: 0 0 0.8rem;
    font-size: 1.05rem;
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
