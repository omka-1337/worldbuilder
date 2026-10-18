<script lang="ts">
  // Marks in a textarea's left padding: a green bar beside lines that are
  // new or changed since the last saved version, a red notch where lines
  // were removed. Lines wrap, so their positions are measured on a hidden
  // copy of the text laid out exactly like the textarea.
  import type { LineChanges } from '../lib/linediff'

  let {
    textarea,
    text,
    changes,
  }: { textarea: HTMLTextAreaElement | undefined; text: string; changes: LineChanges | null } = $props()

  let mirror = $state<HTMLDivElement>()
  let tops = $state<number[]>([]) // top of each line, then the bottom of the last
  let scroll = $state(0)
  let frame = 0

  const copied = [
    'fontFamily', 'fontSize', 'fontWeight', 'fontStyle', 'letterSpacing', 'lineHeight', 'tabSize',
    'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft', 'whiteSpace', 'overflowWrap', 'wordBreak',
  ] as const

  function measure() {
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(() => {
      if (!textarea || !mirror) return
      const style = getComputedStyle(textarea)
      for (const p of copied) mirror.style[p] = style[p]
      mirror.style.width = `${textarea.clientWidth}px`
      const lines = text.split('\n')
      mirror.replaceChildren(
        ...lines.map((line) => {
          const div = document.createElement('div')
          div.textContent = line || '​'
          return div
        }),
      )
      const divs = [...mirror.children] as HTMLElement[]
      tops = [...divs.map((d) => d.offsetTop), divs.length ? divs.at(-1)!.offsetTop + divs.at(-1)!.offsetHeight : 0]
      scroll = textarea.scrollTop
    })
  }

  $effect(() => {
    void text
    if (changes && (changes.added.size || changes.deleted.size)) measure()
  })

  $effect(() => {
    const el = textarea
    if (!el) return
    const onScroll = () => (scroll = el.scrollTop)
    const resized = new ResizeObserver(measure)
    el.addEventListener('scroll', onScroll)
    resized.observe(el)
    return () => {
      el.removeEventListener('scroll', onScroll)
      resized.disconnect()
      cancelAnimationFrame(frame)
    }
  })

  // Runs of consecutive changed lines become one bar.
  const bars = $derived.by(() => {
    if (!changes || tops.length === 0) return []
    const out: { top: number; height: number }[] = []
    const sorted = [...changes.added].sort((a, b) => a - b)
    for (let k = 0; k < sorted.length; k++) {
      const first = sorted[k]
      while (sorted[k + 1] === sorted[k] + 1) k++
      const top = tops[first]
      const bottom = tops[sorted[k] + 1]
      if (top !== undefined && bottom !== undefined) out.push({ top, height: bottom - top })
    }
    return out
  })
  const notches = $derived(
    changes && tops.length ? [...changes.deleted].map((at) => tops[Math.min(at, tops.length - 1)]).filter((y) => y !== undefined) : [],
  )
</script>

<div class="clip" aria-hidden="true">
  <div class="marks" style:transform="translateY({-scroll}px)">
    {#each bars as b (b.top)}
      <span class="added" style:top="{b.top}px" style:height="{b.height}px"></span>
    {/each}
    {#each notches as y (y)}
      <span class="deleted" style:top="{y}px"></span>
    {/each}
  </div>
</div>
<div class="mirror" bind:this={mirror} aria-hidden="true"></div>

<style>
  /* Placed over the textarea's padding box by the parent (inset: border). */
  .clip {
    position: absolute;
    inset: 1px;
    overflow: hidden;
    pointer-events: none;
  }
  .marks {
    position: absolute;
    inset: 0;
  }
  .added {
    position: absolute;
    left: 0.3rem;
    width: 3px;
    border-radius: 2px;
    background: var(--added);
  }
  .deleted {
    position: absolute;
    left: 0.15rem;
    margin-top: -4px;
    width: 0;
    height: 0;
    border-top: 4px solid transparent;
    border-bottom: 4px solid transparent;
    border-left: 6px solid var(--removed);
  }
  .mirror {
    position: absolute;
    top: 0;
    left: 0;
    visibility: hidden;
    pointer-events: none;
    box-sizing: border-box;
    white-space: pre-wrap;
    overflow-wrap: break-word;
  }
</style>
