<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity'
  import Icon from './Icon.svelte'
  import SettingsDialog from './SettingsDialog.svelte'
  import PageTree from './PageTree.svelte'
  import ContextMenu, { type MenuItem } from './ContextMenu.svelte'
  import { api, currentWorld, editHref, HttpError, key, pageHref, worldHref, type Hit, type TreeNode } from '../lib/api'
  import { askName, confirmAction } from '../lib/dialog.svelte'
  import { router, navigate, titleFrom } from '../lib/router.svelte'
  import { store, refresh } from '../lib/store.svelte'
  import { drag, dragging, endDrag } from '../lib/drag.svelte'
  import { prefs, SIDEBAR } from '../lib/prefs.svelte'
  import { changes, articleStates, withDeleted } from '../lib/versions.svelte'

  let query = $state('')
  let hits = $state<Hit[]>([])
  let settingsOpen = $state(false)
  let timer: ReturnType<typeof setTimeout>

  function onInput() {
    clearTimeout(timer)
    const q = query.trim()
    if (!q) {
      hits = []
      return
    }
    timer = setTimeout(async () => {
      const found = await api.search(q, 8)
      if (query.trim() === q) hits = found
    }, 120)
  }

  // Enter goes straight to an exactly matching article, otherwise to results.
  async function onSubmit(e: SubmitEvent) {
    e.preventDefault()
    const q = query.trim()
    if (!q) return
    const found = await api.search(q, 1)
    if (found[0] && key(found[0].title) === key(q)) navigate(pageHref(found[0].title))
    else navigate(worldHref(undefined, '/search?q=' + encodeURIComponent(q)))
    reset()
  }

  function reset() {
    query = ''
    hits = []
  }

  // The article open in the main pane, to highlight it in the tree. The URL
  // may use an alias or other casing, so match it to the real title.
  const current = $derived.by(() => {
    const prefix = new URL(worldHref(undefined, '/wiki/'), location.href).pathname
    if (!router.path.startsWith(prefix)) return ''
    const k = key(titleFrom(router.path.slice(prefix.length)))
    return store.pages.find((p) => key(p.title) === k || p.aliases.some((a) => key(a) === k))?.title ?? ''
  })

  // Open groups (by folder path) are remembered per world in this browser.
  const storageKey = `worldbuilder:tree:${currentWorld()}`
  const expanded = new SvelteSet<string>(loadExpanded())
  function loadExpanded(): string[] {
    try {
      return JSON.parse(localStorage.getItem(storageKey) ?? '[]')
    } catch {
      return []
    }
  }
  $effect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify([...expanded]))
    } catch {
      // Storage unavailable: groups just start closed next time.
    }
  })

  // Opening an article reveals it: every group above it unfolds.
  $effect(() => {
    if (!current) return
    const path: string[] = []
    const find = (nodes: TreeNode[]): boolean =>
      nodes.some((n) => {
        if (n.title === current) return true
        path.push(n.path)
        if (n.children && find(n.children)) return true
        path.pop()
        return false
      })
    if (find(store.tree)) path.forEach((id) => expanded.add(id))
  })

  // --- tree actions --------------------------------------------------------

  let treeError = $state('')
  function fail(e: unknown, conflict: string) {
    treeError =
      e instanceof HttpError && e.status === 409 ? conflict : `Не вдалося: ${e instanceof Error ? e.message : e}`
    setTimeout(() => (treeError = ''), 5000)
  }

  // Groups are remembered by path, so a group that moves or is renamed
  // carries its open state (and its open subgroups') to the new path.
  function remapExpanded(from: string, to: string) {
    if (!from || from === to) return
    for (const p of [...expanded]) {
      if (p === from || p.startsWith(from + '/')) {
        expanded.delete(p)
        expanded.add(to + p.slice(from.length))
      }
    }
  }
  function pathOf(title: string, nodes: TreeNode[] = store.tree): string {
    for (const n of nodes) {
      if (n.title === title) return n.path
      const inner = n.children ? pathOf(title, n.children) : ''
      if (inner) return inner
    }
    return ''
  }

  async function move(title: string, folder: string) {
    const from = pathOf(title)
    try {
      await api.move(title, folder)
      if (folder) expanded.add(folder)
      await refresh()
      remapExpanded(from, pathOf(title))
    } catch (e) {
      fail(e, `Там уже є стаття «${title}».`)
    }
  }

  async function moveFolder(folder: string, to: string) {
    const name = folder.slice(folder.lastIndexOf('/') + 1)
    try {
      await api.moveFolder(folder, to)
      if (to) expanded.add(to)
      await refresh()
      remapExpanded(folder, join(to, name))
    } catch (e) {
      fail(e, `Там уже є папка «${name}».`)
    }
  }

  const join = (folder: string, name: string) => (folder ? `${folder}/${name}` : name)

  function newArticle(folder: string) {
    navigate(worldHref(undefined, '/new' + (folder ? '?folder=' + encodeURIComponent(folder) : '')))
  }

  async function newFolder(folder: string) {
    const name = await askName('Нова папка', '', 'Створити')
    if (!name) return
    try {
      await api.createFolder(join(folder, name))
      if (folder) expanded.add(folder)
      await refresh()
    } catch (e) {
      fail(e, `Папка «${name}» вже існує.`)
    }
  }

  async function renameArticle(title: string) {
    const name = await askName('Перейменувати статтю', title, 'Перейменувати')
    if (!name || name === title) return
    try {
      const from = pathOf(title)
      const newTitle = await api.rename(title, name)
      await refresh()
      remapExpanded(from, pathOf(newTitle))
      if (current === title) navigate(pageHref(newTitle), { replace: true })
    } catch (e) {
      fail(e, `Стаття «${name}» вже існує.`)
    }
  }

  async function renameFolder(node: TreeNode) {
    const name = await askName('Перейменувати папку', node.name, 'Перейменувати')
    if (!name || name === node.name) return
    try {
      await api.renameFolder(node.path, name)
      await refresh()
      const parent = node.path.includes('/') ? node.path.slice(0, node.path.lastIndexOf('/') + 1) : ''
      remapExpanded(node.path, parent + name)
    } catch (e) {
      fail(e, `Папка «${name}» вже існує.`)
    }
  }

  async function deleteArticle(title: string, hasChildren: boolean) {
    const text =
      'Стаття переміститься в кошик світу (прихована папка .trash), звідки її можна відновити.' +
      (hasChildren ? ' Вкладені статті залишаться на місці, у звичайній папці.' : '')
    if (!(await confirmAction(`Видалити «${title}»?`, text, 'Видалити', true))) return
    try {
      await api.deletePage(title)
      await refresh()
      if (current === title) navigate(worldHref(), { replace: true })
    } catch (e) {
      fail(e, '')
    }
  }

  async function deleteFolder(node: TreeNode) {
    const text = 'Папка з усім, що в ній, переміститься в кошик світу (прихована папка .trash), звідки її можна відновити.'
    if (!(await confirmAction(`Видалити папку «${node.name}»?`, text, 'Видалити', true))) return
    try {
      await api.deleteFolder(node.path)
      await refresh()
    } catch (e) {
      fail(e, '')
    }
  }

  // --- context menu ----------------------------------------------------------

  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)

  function openMenu(e: MouseEvent, node: TreeNode | null) {
    let items: MenuItem[]
    if (!node) {
      items = [
        { label: 'Створити статтю', icon: 'plus', run: () => newArticle('') },
        { label: 'Створити папку', icon: 'folder-plus', run: () => newFolder('') },
      ]
    } else if (node.title) {
      const title = node.title
      const nested = node.path.includes('/')
      items = [
        { label: 'Створити статтю', icon: 'plus', run: () => newArticle(node.path) },
        { label: 'Створити папку', icon: 'folder-plus', run: () => newFolder(node.path) },
        'separator',
        { label: 'Редагувати', icon: 'edit', run: () => navigate(editHref(title)) },
        { label: 'Перейменувати', icon: 'pencil', run: () => renameArticle(title) },
        ...(nested ? [{ label: 'Перемістити на верхній рівень', icon: 'to-top' as const, run: () => move(title, '') }] : []),
        'separator',
        { label: 'Видалити', icon: 'trash', danger: true, run: () => deleteArticle(title, !!node.children?.length) },
      ]
    } else {
      items = [
        { label: 'Створити статтю', icon: 'plus', run: () => newArticle(node.path) },
        { label: 'Створити папку', icon: 'folder-plus', run: () => newFolder(node.path) },
        'separator',
        { label: 'Перейменувати', icon: 'pencil', run: () => renameFolder(node) },
        ...(node.path.includes('/')
          ? [{ label: 'Перемістити на верхній рівень', icon: 'to-top' as const, run: () => moveFolder(node.path, '') }]
          : []),
        'separator',
        { label: 'Видалити', icon: 'trash', danger: true, run: () => deleteFolder(node) },
      ]
    }
    menu = { x: e.clientX, y: e.clientY, items }
  }

  let overRoot = $state(false)

  // Articles against the last saved version: colored in the tree, and the
  // deleted ones still shown, struck out.
  const states = $derived(articleStates(changes.list))
  const tree = $derived(withDeleted(store.tree, states))

  // --- width -------------------------------------------------------------

  let resizing = $state(false)
  const clampWidth = (w: number) => Math.round(Math.min(Math.max(w, SIDEBAR.min), SIDEBAR.max))

  function startResize(e: PointerEvent) {
    if (e.button !== 0) return
    e.preventDefault()
    const handle = e.currentTarget as HTMLElement
    handle.setPointerCapture(e.pointerId)
    const startX = e.clientX
    const startWidth = handle.parentElement!.getBoundingClientRect().width
    resizing = true
    const move = (e: PointerEvent) => (prefs.sidebar = clampWidth(startWidth + e.clientX - startX))
    const up = () => {
      resizing = false
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
      handle.removeEventListener('pointercancel', up)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
    handle.addEventListener('pointercancel', up)
  }
</script>

<aside class="sidebar">
  <form class="search" onsubmit={onSubmit}>
    <input
      type="search"
      placeholder="Пошук статей…"
      aria-label="Пошук статей"
      bind:value={query}
      oninput={onInput}
      onkeydown={(e) => e.key === 'Escape' && reset()}
    />
    {#if hits.length}
      <ul class="suggestions">
        {#each hits as hit (hit.title)}
          <li><a href={pageHref(hit.title)} onclick={reset}>{hit.title}</a></li>
        {/each}
      </ul>
    {/if}
  </form>

  <!-- While dragging, the heading turns into the top-level drop zone in
       place, so nothing under the pointer moves. -->
  <div class="tree-head">
    <h2>Статті <span class="muted">{store.pages.length}</span></h2>
    {#if dragging()}
      <div
        class="root-drop"
        class:over={overRoot}
        role="presentation"
        ondragover={(e) => {
          e.preventDefault()
          overRoot = true
        }}
        ondragleave={() => (overRoot = false)}
        ondrop={(e) => {
          e.preventDefault()
          const { title, folder } = drag
          overRoot = false
          endDrag()
          if (title) move(title, '')
          else moveFolder(folder, '')
        }}
      >
        Сюди — на верхній рівень
      </div>
    {/if}
  </div>
  <!-- Right-click on empty space offers creating at the top level. -->
  <div
    class="pages"
    role="tree"
    aria-label="Статті"
    tabindex="-1"
    oncontextmenu={(e) => {
      e.preventDefault()
      openMenu(e, null)
    }}
  >
    {#if treeError}<p class="move-error">{treeError}</p>{/if}
    <PageTree nodes={tree} {states} {current} {expanded} onmove={move} onmovefolder={moveFolder} onmenu={openMenu} />
    {#if store.tree.length > 1 && !dragging()}
      <p class="tip muted">Перетягніть статтю чи папку на іншу статтю чи папку, щоб перемістити. Права кнопка — меню.</p>
    {/if}
  </div>

  <!-- The edge towards the article: drag to resize, double-click to reset;
       arrow keys when focused. -->
  <button
    class="resize"
    class:resizing
    aria-label="Ширина панелі статей: стрілки вліво й вправо"
    title="Перетягніть, щоб змінити ширину; подвійний клік — як було"
    onpointerdown={startResize}
    ondblclick={() => (prefs.sidebar = SIDEBAR.default)}
    onkeydown={(e) => {
      const step = e.key === 'ArrowLeft' ? -16 : e.key === 'ArrowRight' ? 16 : 0
      if (step) {
        e.preventDefault()
        prefs.sidebar = clampWidth(prefs.sidebar + step)
      }
    }}
  ></button>

  <footer>
    <a class="world" href={worldHref()} title="Головна сторінка світу">{store.world?.name ?? ''}</a>
    <button class="icon" onclick={() => (settingsOpen = true)} title="Налаштування" aria-label="Налаштування">
      <Icon name="settings" size={18} />
    </button>
    <a class="icon" href="/" title="Вийти зі світу" aria-label="Вийти зі світу">
      <Icon name="exit" size={18} />
    </a>
  </footer>
</aside>

<SettingsDialog bind:open={settingsOpen} />
{#if menu}<ContextMenu {...menu} onclose={() => (menu = null)} />{/if}

<style>
  .sidebar {
    position: sticky;
    top: 0;
    height: 100vh;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    padding: 0.75rem 0.75rem 0;
    background: var(--bg-sidebar);
    border-inline: 1px solid var(--border);
    overflow: hidden;
  }
  .search {
    position: relative;
  }
  .resize {
    position: absolute;
    top: 0;
    bottom: 0;
    right: 0;
    z-index: 5;
    width: 7px; /* the sidebar clips overflow, so the grip stays inside */
    padding: 0;
    border: none;
    border-radius: 0;
    background: none;
    cursor: col-resize;
    touch-action: none;
  }
  .resize::after {
    content: '';
    position: absolute;
    inset: 0 0 0 4px;
    transition: background 0.15s;
  }
  .resize:hover::after,
  .resize:focus-visible::after,
  .resize.resizing::after {
    background: var(--accent);
  }
  .resize:focus-visible {
    outline: none;
  }
  .resize:hover {
    background: none;
  }
  .search input {
    width: 100%;
    padding: 0.4rem 0.65rem;
  }
  .suggestions {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    z-index: 5;
    list-style: none;
    margin: 0;
    padding: 0.25rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.12);
  }
  .suggestions a {
    display: block;
    padding: 0.3rem 0.5rem;
    border-radius: 4px;
    color: var(--text);
  }
  .suggestions a:hover {
    background: var(--bg-hover);
    text-decoration: none;
  }
  h2 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-muted);
    margin: 0.5rem 0.5rem 0;
  }
  .pages {
    overflow: auto;
    flex: 1;
    font-size: 0.93rem;
  }
  .tree-head {
    position: relative;
    display: flex;
    align-items: flex-end;
    min-height: 2.1rem;
  }
  .root-drop {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    border: 2px dashed var(--border);
    border-radius: 6px;
    background: var(--bg-sidebar);
    font-size: 0.82rem;
    color: var(--text-muted);
  }
  .root-drop.over {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent);
  }
  .move-error {
    margin: 0 0 0.3rem;
    font-size: 0.82rem;
    color: var(--danger);
  }
  .tip {
    margin: 0.8rem 0.4rem;
    font-size: 0.78rem;
  }

  footer {
    display: flex;
    align-items: center;
    gap: 0.15rem;
    margin: 0 -0.75rem;
    padding: 0.55rem 0.6rem 0.55rem 1.25rem;
    border-top: 1px solid var(--border);
  }
  .world {
    flex: 1;
    min-width: 0;
    font-weight: 700;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .icon {
    display: grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    padding: 0;
    border: none;
    border-radius: 6px;
    background: none;
    color: var(--text-muted);
  }
  .icon:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
</style>
