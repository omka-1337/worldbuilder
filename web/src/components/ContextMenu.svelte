<script lang="ts" module>
  import type { IconName } from './Icon.svelte'
  export type MenuItem = { label: string; icon?: IconName; danger?: boolean; run: () => void } | 'separator'
</script>

<script lang="ts">
  import { tick } from 'svelte'
  import Icon from './Icon.svelte'

  // A right-click menu at a screen position. It closes on any outside click,
  // Escape, scrolling or resizing; arrow keys move between items.
  let { x, y, items, onclose }: { x: number; y: number; items: MenuItem[]; onclose: () => void } = $props()

  let el = $state<HTMLUListElement>()
  let left = $state(0)
  let top = $state(0)

  $effect(() => {
    // Keep the menu on screen near the edges.
    void x
    void y
    tick().then(() => {
      if (!el) return
      const r = el.getBoundingClientRect()
      left = Math.min(x, innerWidth - r.width - 6)
      top = Math.min(y, innerHeight - r.height - 6)
      el.querySelector<HTMLButtonElement>('button')?.focus()
    })
  })

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault()
      onclose()
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const buttons = [...el!.querySelectorAll<HTMLButtonElement>('button')]
      const i = buttons.indexOf(document.activeElement as HTMLButtonElement)
      buttons[(i + (e.key === 'ArrowDown' ? 1 : buttons.length - 1)) % buttons.length]?.focus()
    }
  }
</script>

<svelte:window
  onmousedown={(e) => !el?.contains(e.target as Node) && onclose()}
  onresize={onclose}
  onblur={onclose}
/>

<ul class="menu" role="menu" bind:this={el} style:left="{left}px" style:top="{top}px" onkeydown={onKeydown}>
  {#each items as item, i (i)}
    {#if item === 'separator'}
      <li class="sep" role="separator"></li>
    {:else}
      <li role="none">
        <button
          role="menuitem"
          class:danger={item.danger}
          onclick={() => {
            onclose()
            item.run()
          }}
        >
          {#if item.icon}<Icon name={item.icon} size={15} />{:else}<span class="no-icon"></span>{/if}
          {item.label}
        </button>
      </li>
    {/if}
  {/each}
</ul>

<style>
  .menu {
    position: fixed;
    z-index: 60;
    min-width: 13rem;
    margin: 0;
    padding: 0.3rem;
    list-style: none;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 10px 28px rgb(0 0 0 / 0.18);
    font-size: 0.9rem;
  }
  button {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    width: 100%;
    padding: 0.35rem 0.6rem;
    border: none;
    border-radius: 5px;
    background: none;
    text-align: left;
    color: var(--text);
  }
  button:hover,
  button:focus-visible {
    background: var(--accent-soft);
    color: var(--accent);
    outline: none;
  }
  button.danger {
    color: var(--danger);
  }
  button.danger:hover,
  button.danger:focus-visible {
    background: color-mix(in srgb, var(--danger) 12%, transparent);
    color: var(--danger);
  }
  .no-icon {
    width: 15px;
  }
  .sep {
    height: 1px;
    margin: 0.3rem 0.2rem;
    background: var(--border);
  }
</style>
