<script lang="ts">
  import { api, HttpError } from '../lib/api'

  // The infobox picture: drop a file, pick one, or paste from the clipboard.
  // `value` is what the article stores (![[name.png]]), `url` the preview.
  let {
    value = $bindable(''),
    url = $bindable(''),
    title,
  }: { value?: string; url?: string; title: string } = $props()

  let input = $state<HTMLInputElement>()
  let over = $state(false)
  let busy = $state(false)
  let error = $state('')

  async function upload(file: File | undefined | null) {
    if (!file) return
    if (!title.trim()) {
      error = 'Спершу дайте статті назву.'
      return
    }
    if (!/^image\/(png|jpeg|gif|webp)$/.test(file.type)) {
      error = 'Підходять зображення PNG, JPEG, GIF або WebP.'
      return
    }
    busy = true
    error = ''
    try {
      const saved = await api.uploadImage(title.trim(), file)
      value = `![[${saved.name}]]`
      url = saved.url
    } catch (e) {
      error =
        e instanceof HttpError && e.status === 413
          ? 'Зображення завелике (більше 20 МБ).'
          : `Не вдалося завантажити: ${e instanceof Error ? e.message : e}`
    } finally {
      busy = false
    }
  }

  function onPaste(e: ClipboardEvent) {
    const file = [...(e.clipboardData?.files ?? [])].find((f) => f.type.startsWith('image/'))
    if (file) {
      e.preventDefault()
      upload(file)
    }
  }
</script>

<div
  class="picker"
  class:over
  class:has={!!value}
  role="button"
  tabindex="0"
  aria-label={value ? 'Зображення інфобокса' : 'Додати зображення'}
  onclick={() => !value && input?.click()}
  onkeydown={(e) => !value && (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), input?.click())}
  onpaste={onPaste}
  ondragover={(e) => {
    if (!e.dataTransfer?.types.includes('Files')) return
    e.preventDefault()
    over = true
  }}
  ondragleave={() => (over = false)}
  ondrop={(e) => {
    e.preventDefault()
    over = false
    upload(e.dataTransfer?.files[0])
  }}
>
  {#if busy}
    <span class="muted">Завантаження…</span>
  {:else if value && url}
    <img src={url} alt="" />
  {:else if value}
    <span class="muted">Файл {value} не знайдено</span>
  {:else}
    <span class="hint">
      <strong>Додати зображення</strong>
      <span class="muted">Перетягніть файл сюди, натисніть, щоб вибрати, або вставте Ctrl+V</span>
    </span>
  {/if}
</div>

{#if value && !busy}
  <div class="actions">
    <button type="button" onclick={() => input?.click()}>Замінити</button>
    <button type="button" onclick={() => ((value = ''), (url = ''))}>Прибрати</button>
  </div>
{/if}
{#if error}<p class="error">{error}</p>{/if}

<input
  bind:this={input}
  type="file"
  accept="image/png,image/jpeg,image/gif,image/webp"
  hidden
  onchange={(e) => {
    upload(e.currentTarget.files?.[0])
    e.currentTarget.value = ''
  }}
/>

<style>
  .picker {
    display: grid;
    place-items: center;
    min-height: 6.5rem;
    border: 2px dashed var(--border);
    border-radius: 8px;
    text-align: center;
    cursor: pointer;
    overflow: hidden;
  }
  .picker.has {
    border-style: solid;
    border-width: 1px;
    cursor: default;
  }
  .picker:hover:not(.has),
  .picker:focus-visible,
  .picker.over {
    border-color: var(--accent);
    background: var(--accent-soft);
    outline: none;
  }
  .hint {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    padding: 0.8rem;
    font-size: 0.85rem;
  }
  .hint strong {
    color: var(--accent);
  }
  img {
    display: block;
    width: 100%;
    max-height: 16rem;
    object-fit: contain;
  }
  .actions {
    display: flex;
    gap: 0.4rem;
  }
  .actions button {
    padding: 0.2rem 0.6rem;
    font-size: 0.85rem;
  }
  .error {
    margin: 0;
    font-size: 0.8rem;
    color: var(--danger);
  }
</style>
