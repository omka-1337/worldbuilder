<script lang="ts">
  import type { SvelteSet } from 'svelte/reactivity'
  import Self from './PageTree.svelte'
  import Icon from './Icon.svelte'
  import { pageHref, type TreeNode } from '../lib/api'
  import { drag, startDrag, endDrag, accepts } from '../lib/drag.svelte'
  import { typeColor } from '../lib/colors'

  let {
    nodes,
    depth = 0,
    current,
    expanded,
    onmove,
    onmenu,
  }: {
    nodes: TreeNode[]
    depth?: number
    current: string // title of the open article
    expanded: SvelteSet<string> // paths of open groups
    onmove: (title: string, folder: string) => void
    onmenu: (e: MouseEvent, node: TreeNode) => void
  } = $props()

  const toggle = (n: TreeNode) => (expanded.has(n.path) ? expanded.delete(n.path) : expanded.add(n.path))

  // Hovering a closed group while dragging opens it after a moment.
  let openTimer: ReturnType<typeof setTimeout>

  function onDragOver(e: DragEvent, n: TreeNode) {
    if (!accepts(n)) return
    e.preventDefault()
    e.stopPropagation()
    e.dataTransfer!.dropEffect = 'move'
    if (drag.over !== n.path) {
      drag.over = n.path
      clearTimeout(openTimer)
      if (n.children?.length && !expanded.has(n.path)) openTimer = setTimeout(() => expanded.add(n.path), 700)
    }
  }

  function onDrop(e: DragEvent, n: TreeNode) {
    if (!accepts(n)) return
    e.preventDefault()
    e.stopPropagation()
    clearTimeout(openTimer)
    const title = drag.title
    endDrag()
    onmove(title, n.path)
  }
</script>

<ul class="tree">
  {#each nodes as n (n.path)}
    {@const open = expanded.has(n.path)}
    {@const folder = !n.title}
    <li>
      <div
        class="row"
        class:active={!!n.title && n.title === current}
        class:over={drag.over === n.path}
        class:dragged={!!n.title && drag.title === n.title}
        style:padding-left="{depth * 0.9 + 0.15}rem"
        draggable={!folder}
        role="treeitem"
        aria-selected={!!n.title && n.title === current}
        aria-expanded={n.children?.length ? open : undefined}
        tabindex="-1"
        ondragstart={(e) => {
          e.dataTransfer!.effectAllowed = 'move'
          e.dataTransfer!.setData('text/plain', n.title ?? '')
          startDrag(n)
        }}
        ondragend={endDrag}
        ondragover={(e) => onDragOver(e, n)}
        ondragleave={() => drag.over === n.path && (drag.over = '')}
        ondrop={(e) => onDrop(e, n)}
        oncontextmenu={(e) => {
          e.preventDefault()
          e.stopPropagation()
          onmenu(e, n)
        }}
      >
        {#if n.children?.length}
          <button class="chevron" class:open onclick={() => toggle(n)} aria-label={open ? 'Згорнути' : 'Розгорнути'}>
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="m9 6 6 6-6 6" /></svg>
          </button>
        {:else}
          <span class="chevron-space"></span>
        {/if}
        {#if folder}
          <button class="name folder" onclick={() => toggle(n)}>
            <span class="icon"><Icon name={open && n.children?.length ? 'folder-open' : 'folder'} size={15} /></span>
            <span class="text">{n.name}</span>
          </button>
        {:else}
          <a class="name" href={pageHref(n.title!)} draggable="false">
            <span class="icon" style:color={n.type ? typeColor(n.type) : undefined}><Icon name="file" size={15} /></span>
            <span class="text">{n.name}</span>
          </a>
        {/if}
      </div>
      {#if open && n.children?.length}
        <Self nodes={n.children} depth={depth + 1} {current} {expanded} {onmove} {onmenu} />
      {/if}
    </li>
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.1rem;
    border-radius: 5px;
    outline: none;
  }
  .row:hover {
    background: var(--bg-hover);
  }
  .row.active {
    background: var(--accent-soft);
  }
  .row.active .name {
    color: var(--accent);
  }
  .row.over {
    background: var(--accent-soft);
    box-shadow: inset 0 0 0 2px var(--accent);
  }
  .row.dragged {
    opacity: 0.45;
  }
  .name {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.2rem 0.4rem 0.2rem 0.1rem;
    color: var(--text);
  }
  a.name:hover {
    text-decoration: none;
  }
  .text {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .icon {
    display: grid;
    flex: none;
    color: var(--text-muted);
  }
  .folder {
    border: none;
    background: none;
    text-align: left;
    font: inherit;
  }
  .folder:hover {
    background: none;
  }
  .chevron,
  .chevron-space {
    flex: none;
    width: 1.2rem;
    height: 1.2rem;
  }
  .chevron {
    display: grid;
    place-items: center;
    padding: 0;
    border: none;
    background: none;
    color: var(--text-muted);
    border-radius: 4px;
  }
  .chevron:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .chevron svg {
    transition: transform 0.15s;
  }
  .chevron.open svg {
    transform: rotate(90deg);
  }
</style>
