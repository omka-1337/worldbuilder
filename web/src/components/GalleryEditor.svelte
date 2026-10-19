<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, HttpError } from '../lib/api'
  import { linkComplete } from '../lib/linkComplete'
  import { store } from '../lib/store.svelte'

  // The article's gallery in the editor: pictures with captions, in order.
  // Several files can be dropped, picked or pasted at once; each is stored
  // in the article's folder like the infobox picture.
  type Item = { file: string; caption: string; url: string }
  let { items = $bindable([]), title }: { items: Item[]; title: string } = $props()

  let input = $state<HTMLInputElement>()
  let over = $state(false)
  let busy = $state(0) // uploads under way
  let error = $state('')
  const pages = () => store.pages

  async function upload(files: FileList | File[] | undefined | null) {
    const list = [...(files ?? [])].filter((f) => f.type.startsWith('image/'))
    if (!list.length) return
    if (!title.trim()) {
      error = 'Спершу дайте статті назву.'
      return
    }
    error = ''
    for (const file of list) {
      if (!/^image\/(png|jpeg|gif|webp)$/.test(file.type)) {
        error = 'Підходять зображення PNG, JPEG, GIF або WebP.'
        continue
      }
      busy++
      try {
        const saved = await api.uploadImage(title.trim(), file)
        items.push({ file: `![[${saved.name}]]`, caption: '', url: saved.url })
      } catch (e) {
        error =
          e instanceof HttpError && e.status === 413
            ? 'Зображення завелике (більше 20 МБ).'
            : `Не вдалося завантажити: ${e instanceof Error ? e.message : e}`
      } finally {
        busy--
      }
    }
  }

  function move(i: number, dir: -1 | 1) {
    const j = i + dir
    if (j >= 0 && j < items.length) [items[i], items[j]] = [items[j], items[i]]
  }
</script>

{#if items.length}
  <ol class="items">
    {#each items as item, i (i)}
      <li>
        <div class="thumb">
          {#if item.url}<img src={item.url} alt="" />{:else}<span class="muted" title={item.file}>?</span>{/if}
        </div>
        <div class="side">
          <input
            bind:value={item.caption}
            use:linkComplete={pages}
            placeholder="Підпис"
            aria-label="Підпис до фото {i + 1}"
            autocomplete="off"
          />
          <div class="tools">
            <button class="icon" onclick={() => move(i, -1)} disabled={i === 0} title="Раніше" aria-label="Перемістити фото раніше">
              <Icon name="up" size={14} />
            </button>
            <button class="icon" onclick={() => move(i, 1)} disabled={i === items.length - 1} title="Пізніше" aria-label="Перемістити фото пізніше">
              <Icon name="down" size={14} />
            </button>
            <button class="icon" onclick={() => items.splice(i, 1)} title="Прибрати з галереї" aria-label="Прибрати фото з галереї">
              <Icon name="trash" size={14} />
            </button>
          </div>
        </div>
      </li>
    {/each}
  </ol>
{/if}

<div
  class="drop"
  class:over
  role="button"
  tabindex="0"
  aria-label="Додати фото до галереї"
  onclick={() => input?.click()}
  onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), input?.click())}
  onpaste={(e) => {
    const files = [...(e.clipboardData?.files ?? [])]
    if (files.some((f) => f.type.startsWith('image/'))) {
      e.preventDefault()
      upload(files)
    }
  }}
  ondragover={(e) => {
    if (!e.dataTransfer?.types.includes('Files')) return
    e.preventDefault()
    over = true
  }}
  ondragleave={() => (over = false)}
  ondrop={(e) => {
    e.preventDefault()
    over = false
    upload(e.dataTransfer?.files)
  }}
>
  {#if busy}
    <span class="muted">Завантаження…</span>
  {:else}
    <strong>+ Додати фото</strong>
    <span class="muted">Перетягніть файли сюди, натисніть, щоб вибрати, або вставте Ctrl+V</span>
  {/if}
</div>
{#if error}<p class="error">{error}</p>{/if}

<input
  bind:this={input}
  type="file"
  accept="image/png,image/jpeg,image/gif,image/webp"
  multiple
  hidden
  onchange={(e) => {
    upload(e.currentTarget.files)
    e.currentTarget.value = ''
  }}
/>

<style>
  .items {
    list-style: none;
    margin: 0 0 0.5rem;
    padding: 0;
    display: grid;
    gap: 0.5rem;
  }
  li {
    display: grid;
    grid-template-columns: 4.5rem minmax(0, 1fr);
    gap: 0.5rem;
    align-items: center;
  }
  .thumb {
    display: grid;
    place-items: center;
    aspect-ratio: 4 / 3;
    border-radius: 6px;
    overflow: hidden;
    background: var(--bg-sidebar);
  }
  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .side {
    display: grid;
    gap: 0.2rem;
  }
  .side input {
    width: 100%;
    padding: 0.3rem 0.5rem;
    font-size: 0.88rem;
  }
  .tools {
    display: flex;
    justify-content: flex-end;
  }
  .icon {
    display: grid;
    place-items: center;
    padding: 0.2rem;
    border-color: transparent;
    background: none;
    color: var(--text-muted);
  }
  .icon:hover:not(:disabled) {
    color: var(--text);
  }
  .drop {
    display: grid;
    gap: 0.15rem;
    padding: 0.6rem;
    border: 2px dashed var(--border);
    border-radius: 8px;
    text-align: center;
    font-size: 0.82rem;
    cursor: pointer;
  }
  .drop strong {
    color: var(--accent);
  }
  .drop:hover,
  .drop:focus-visible,
  .drop.over {
    border-color: var(--accent);
    background: var(--accent-soft);
    outline: none;
  }
  .error {
    margin: 0.3rem 0 0;
    font-size: 0.82rem;
    color: var(--danger);
  }
</style>
