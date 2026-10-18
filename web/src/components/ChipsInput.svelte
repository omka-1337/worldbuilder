<script lang="ts">
  // A list of short values (categories, other names) edited as removable
  // chips. Enter or comma adds what was typed; Backspace on an empty input
  // removes the last chip.
  let {
    values = $bindable([]),
    suggestions = [],
    placeholder = '',
    id,
  }: { values?: string[]; suggestions?: string[]; placeholder?: string; id: string } = $props()

  let text = $state('')
  const lower = $derived(new Set(values.map((v) => v.toLowerCase())))
  const options = $derived(suggestions.filter((s) => !lower.has(s.toLowerCase())))

  function add() {
    const v = text.replace(/,$/, '').trim()
    if (v && !lower.has(v.toLowerCase())) values = [...values, v]
    text = ''
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      add()
    } else if (e.key === 'Backspace' && !text && values.length) {
      values = values.slice(0, -1)
    }
  }

  // Picking from the suggestion list fires input without a key press.
  function onInput() {
    if (options.includes(text)) add()
  }
</script>

<div class="chips">
  {#each values as v, i (v)}
    <span class="chip">
      {v}
      <button type="button" onclick={() => (values = values.filter((_, j) => j !== i))} aria-label="Прибрати «{v}»">×</button>
    </span>
  {/each}
  <input
    {id}
    bind:value={text}
    onkeydown={onKeydown}
    oninput={onInput}
    onblur={add}
    list={options.length ? `${id}-options` : undefined}
    placeholder={values.length ? '' : placeholder}
  />
  {#if options.length}
    <datalist id="{id}-options">
      {#each options as o (o)}<option value={o}></option>{/each}
    </datalist>
  {/if}
</div>

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    padding: 0.3rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg-raised);
  }
  .chips:focus-within {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.15rem;
    padding: 0.05rem 0.2rem 0.05rem 0.55rem;
    border-radius: 999px;
    background: var(--accent-soft);
    color: var(--accent);
    font-size: 0.88rem;
  }
  .chip button {
    border: none;
    background: none;
    padding: 0 0.3rem;
    color: inherit;
    font-size: 1rem;
    line-height: 1;
  }
  input {
    flex: 1;
    min-width: 6rem;
    border: none;
    outline: none;
    padding: 0.15rem 0.3rem;
    background: none;
  }
</style>
