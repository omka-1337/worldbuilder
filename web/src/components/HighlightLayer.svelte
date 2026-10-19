<script lang="ts">
  // The textarea's text, coloured by its Markdown, laid out under the
  // textarea: the textarea's own letters are transparent, so its caret,
  // selection, spelling marks and editing stay as they are, over this copy.
  import { highlightMarkdown } from '../lib/mdhighlight'

  let { textarea, text }: { textarea: HTMLTextAreaElement | undefined; text: string } = $props()

  let layer = $state<HTMLDivElement>()
  let width = $state(0)
  let height = $state(0)
  let scrollTop = $state(0)
  let scrollLeft = $state(0)
  const html = $derived(highlightMarkdown(text))

  const copied = [
    'fontFamily', 'fontSize', 'fontWeight', 'fontStyle', 'letterSpacing', 'lineHeight', 'tabSize', 'wordSpacing',
    'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft', 'whiteSpace', 'overflowWrap', 'wordBreak', 'textIndent',
  ] as const

  // Same box and same type as the textarea's text, and the same scroll.
  $effect(() => {
    const el = textarea
    if (!el || !layer) return
    const sync = () => {
      const style = getComputedStyle(el)
      for (const p of copied) layer!.style[p] = style[p]
      width = el.clientWidth
      height = el.clientHeight
      scrollTop = el.scrollTop
      scrollLeft = el.scrollLeft
    }
    sync()
    const onScroll = () => ((scrollTop = el.scrollTop), (scrollLeft = el.scrollLeft))
    const resized = new ResizeObserver(sync)
    resized.observe(el)
    el.addEventListener('scroll', onScroll)
    return () => {
      resized.disconnect()
      el.removeEventListener('scroll', onScroll)
    }
  })
</script>

<div class="clip" aria-hidden="true" style:width="{width}px" style:height="{height}px">
  <div class="layer" bind:this={layer} style:transform="translate({-scrollLeft}px, {-scrollTop}px)">
    {@html html}
  </div>
</div>

<style>
  /* Over the textarea's padding box (inset by its 1px border), under its
     transparent letters. */
  .clip {
    position: absolute;
    top: 1px;
    left: 1px;
    overflow: hidden;
    pointer-events: none;
    z-index: 0;
  }
  .layer {
    white-space: pre-wrap;
    overflow-wrap: break-word;
    color: var(--text);
  }
  .layer :global(.md-mark),
  .layer :global(.md-quote) {
    color: var(--text-muted);
    opacity: 0.75;
  }
  .layer :global(.md-heading) {
    color: var(--accent);
  }
  .layer :global(.md-heading .md-link) {
    color: inherit;
  }
  .layer :global(.md-link) {
    color: var(--link);
  }
  .layer :global(.md-label) {
    color: var(--text);
  }
  .layer :global(.md-embed),
  .layer :global(.md-url) {
    color: light-dark(#2f8a8a, #6cc4c4);
  }
  .layer :global(.md-date) {
    color: var(--changed);
  }
  .layer :global(.md-note) {
    color: var(--text-muted);
  }
  .layer :global(.md-strong) {
    color: color-mix(in srgb, var(--text) 85%, var(--accent));
    text-shadow: 0.35px 0 0 currentColor; /* a touch heavier, without changing widths */
  }
  .layer :global(.md-em) {
    color: color-mix(in srgb, var(--text) 70%, var(--accent));
  }
  .layer :global(.md-strike) {
    text-decoration: line-through;
    color: var(--text-muted);
  }
  .layer :global(.md-highlight) {
    background: var(--mark);
  }
  .layer :global(.md-code) {
    color: light-dark(#b0503a, #e8917a);
    background: color-mix(in srgb, var(--text) 6%, transparent);
  }
  .layer :global(.md-list) {
    color: var(--accent);
  }
  .layer :global(.md-callout) {
    color: var(--changed);
  }
</style>
