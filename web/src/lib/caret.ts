// Pixel position of the caret in a textarea or input, relative to the
// viewport. Browsers do not expose it, so a hidden copy of the field with the
// same text metrics is laid out and a marker placed where the caret is.

const COPIED = [
  'boxSizing', 'width', 'height', 'overflowX', 'overflowY',
  'borderTopWidth', 'borderRightWidth', 'borderBottomWidth', 'borderLeftWidth', 'borderStyle',
  'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft',
  'fontStyle', 'fontVariant', 'fontWeight', 'fontStretch', 'fontSize', 'lineHeight', 'fontFamily',
  'textAlign', 'textTransform', 'textIndent', 'letterSpacing', 'wordSpacing', 'tabSize',
] as const

export function caretRect(el: HTMLTextAreaElement | HTMLInputElement, pos: number) {
  const style = getComputedStyle(el)
  const mirror = document.createElement('div')
  for (const prop of COPIED) mirror.style[prop] = style[prop]
  Object.assign(mirror.style, {
    position: 'absolute',
    visibility: 'hidden',
    top: '0',
    left: '-9999px',
    whiteSpace: el instanceof HTMLInputElement ? 'pre' : 'pre-wrap',
    overflowWrap: 'break-word',
  })
  mirror.textContent = el.value.slice(0, pos)
  const marker = document.createElement('span')
  marker.textContent = el.value.slice(pos) || '.'
  mirror.append(marker)
  document.body.append(mirror)

  const box = el.getBoundingClientRect()
  const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.3
  const rect = {
    left: box.left + marker.offsetLeft - el.scrollLeft,
    top: box.top + marker.offsetTop - el.scrollTop,
    height: lineHeight,
  }
  mirror.remove()
  return rect
}
