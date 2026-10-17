<script lang="ts">
  import { editHref, key, pageHref } from '../lib/api'
  import { navigate } from '../lib/router.svelte'
  import { store } from '../lib/store.svelte'

  let title = $state('')
  const existing = $derived(store.pages.find((p) => key(p.title) === key(title)))

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (title.trim() && !existing) navigate(editHref(title.trim()))
  }
</script>

<h1 class="page-title">Нова стаття</h1>
<form onsubmit={submit}>
  <!-- svelte-ignore a11y_autofocus -->
  <input bind:value={title} placeholder="Назва статті" autofocus />
  <button class="primary" disabled={!title.trim() || !!existing}>Далі</button>
</form>
{#if existing}
  <p class="muted">Стаття <a href={pageHref(existing.title)}>{existing.title}</a> вже існує.</p>
{/if}

<style>
  form {
    display: flex;
    gap: 0.5rem;
    max-width: 32rem;
    margin-top: 1rem;
  }
  input {
    flex: 1;
    padding: 0.4rem 0.7rem;
  }
</style>
