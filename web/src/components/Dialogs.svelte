<script lang="ts">
  import { tick } from 'svelte'
  import { dialog, answer } from '../lib/dialog.svelte'

  let el = $state<HTMLDialogElement>()
  let input = $state<HTMLInputElement>()
  let value = $state('')

  $effect(() => {
    const r = dialog.current
    if (!el) return
    if (r && !el.open) {
      value = r.value ?? ''
      el.showModal()
      tick().then(() => input?.select())
    } else if (!r && el.open) {
      el.close()
    }
  })

  function submit(e: SubmitEvent) {
    e.preventDefault()
    answer(dialog.current?.value !== undefined ? value : true)
  }
</script>

<dialog bind:this={el} onclose={() => dialog.current && answer(null)} onclick={(e) => e.target === el && answer(null)}>
  {#if dialog.current}
    <form onsubmit={submit}>
      <h2>{dialog.current.title}</h2>
      {#if dialog.current.text}<p>{dialog.current.text}</p>{/if}
      {#if dialog.current.value !== undefined}
        <input bind:this={input} bind:value aria-label={dialog.current.title} />
      {/if}
      <div class="actions">
        <button type="button" onclick={() => answer(null)}>Скасувати</button>
        <button class="primary" class:danger={dialog.current.danger}>{dialog.current.ok}</button>
      </div>
    </form>
  {/if}
</dialog>

<style>
  dialog {
    width: min(26rem, calc(100vw - 2rem));
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
    margin: 0 0 0.6rem;
    font-size: 1.05rem;
  }
  p {
    margin: 0 0 0.8rem;
    font-size: 0.92rem;
    color: var(--text-muted);
  }
  input {
    width: 100%;
    padding: 0.45rem 0.65rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  .primary.danger {
    background: var(--danger);
    border-color: var(--danger);
  }
</style>
