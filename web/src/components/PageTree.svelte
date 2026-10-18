<script lang="ts">
  import type { SvelteSet } from 'svelte/reactivity'
  import Self from './PageTree.svelte'
  import { pageHref, type TreeNode } from '../lib/api'
  import { drag, startDrag, endDrag, accepts } from '../lib/drag.svelte'

  let {
    nodes,
    depth = 0,
    current,
    expanded,
    onmove,
  }: {
    nodes: TreeNode[]
    depth?: number
    current: string // title of the open article
    expanded: SvelteSet<string>
    onmove: (title: string, parent: string) => void
  } = $props()

  const id = (n: TreeNode) => n.title ?? 'folder:' + n.name

  // Hovering a collapsed group while dragging opens it after a moment.
  let openTimer: ReturnType<typeof setTimeout>

  function onDragOver(e: DragEvent, n: TreeNode) {
    if (!accepts(n)) return
    e.preventDefault()
    e.dataTransfer!.dropEffect = 'move'
    if (drag.over !== n.title) {
      drag.over = n.title!
      clearTimeout(openTimer)
      if (n.children?.length && !expanded.has(id(n))) openTimer = setTimeout(() => expanded.add(id(n)), 700)
    }
  }

  function onDrop(e: DragEvent, n: TreeNode) {
    if (!accepts(n)) return
    e.preventDefault()
    clearTimeout(openTimer)
    const title = drag.title
    endDrag()
    onmove(title, n.title!)
  }
</script>

<ul class="tree">
  {#each nodes as n (id(n))}
    {@const open = expanded.has(id(n))}
    <li>
      <div
        class="row"
        class:active={!!n.title && n.title === current}
        class:over={drag.over === n.title && !!n.title}
        class:dragged={drag.title === n.title && !!n.title}
        style:padding-left="{depth * 0.9 + 0.15}rem"
        draggable={!!n.title}
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
        ondragleave={() => drag.over === n.title && (drag.over = '')}
        ondrop={(e) => onDrop(e, n)}
      >
        {#if n.children?.length}
          <button
            class="chevron"
            class:open
            onclick={() => (open ? expanded.delete(id(n)) : expanded.add(id(n)))}
            aria-label={open ? 'Згорнути' : 'Розгорнути'}
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="m9 6 6 6-6 6" /></svg>
          </button>
        {:else}
          <span class="chevron-space"></span>
        {/if}
        {#if n.title}
          <a href={pageHref(n.title)} draggable="false">{n.name}</a>
        {:else}
          <button class="folder" onclick={() => (open ? expanded.delete(id(n)) : expanded.add(id(n)))}>{n.name}</button>
        {/if}
      </div>
      {#if open && n.children?.length}
        <Self nodes={n.children} depth={depth + 1} {current} {expanded} {onmove} />
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
  .row.active a {
    color: var(--accent);
  }
  .row.over {
    background: var(--accent-soft);
    box-shadow: inset 0 0 0 2px var(--accent);
  }
  .row.dragged {
    opacity: 0.45;
  }
  a,
  .folder {
    flex: 1;
    min-width: 0;
    padding: 0.2rem 0.4rem 0.2rem 0.15rem;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  a:hover {
    text-decoration: none;
  }
  .folder {
    border: none;
    background: none;
    text-align: left;
    font: inherit;
    color: var(--text-muted);
    font-style: italic;
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
