// Suggests article titles while a [[link]] is being typed in a textarea or
// input. Use as an action: <textarea use:linkComplete={() => store.pages}>.
//
// Typing "[[" opens a list filtered by what follows it; ↑/↓ pick, Enter or
// Tab inserts the title and closes the link, Esc dismisses. Aliases are
// offered too, and when nothing matches the typed text can still become a
// link to a new (red) article.

import { caretRect } from './caret'
import type { Summary } from './api'

type Field = HTMLTextAreaElement | HTMLInputElement
type Item = { insert: string; title: string; alias?: string; create?: boolean }

const MAX = 8

export function linkComplete(node: Field, pages: () => Summary[]) {
  let getPages = pages
  let items: Item[] = []
  let active = 0
  let start = -1 // index just after "[[" of the link being typed
  const popup = document.createElement('ul')
  popup.className = 'link-complete'
  popup.setAttribute('role', 'listbox')

  // The open link before the caret, if any: "[[" with no "]]", "|", "#" or
  // line break after it.
  function openLink(): { from: number; query: string } | null {
    const caret = node.selectionStart ?? 0
    if (caret !== node.selectionEnd) return null
    const before = node.value.slice(0, caret)
    const from = before.lastIndexOf('[[')
    if (from < 0) return null
    const query = before.slice(from + 2)
    if (/[\]|#\n]/.test(query) || query.length > 60) return null
    return { from: from + 2, query }
  }

  function suggest(query: string): Item[] {
    const q = query.trim().toLowerCase()
    const scored: [number, Item][] = []
    for (const p of getPages()) {
      const names: { name: string; alias?: string }[] = [{ name: p.title }, ...p.aliases.map((a) => ({ name: a, alias: a }))]
      for (const { name, alias } of names) {
        const i = name.toLowerCase().indexOf(q)
        if (i < 0) continue
        // Prefix matches first, then titles before aliases, then shorter names.
        const score = (i === 0 ? 0 : 1000) + (alias ? 100 : 0) + name.length
        scored.push([score, { insert: name, title: p.title, alias }])
      }
    }
    scored.sort((a, b) => a[0] - b[0] || a[1].insert.localeCompare(b[1].insert))
    const out = scored.slice(0, MAX).map(([, item]) => item)
    const typed = query.trim()
    if (typed && !out.some((it) => it.insert.toLowerCase() === typed.toLowerCase())) {
      out.push({ insert: typed, title: typed, create: true })
    }
    return out
  }

  function update() {
    const link = openLink()
    if (!link) return close()
    start = link.from
    items = suggest(link.query)
    if (!items.length) return close()
    active = Math.min(active, items.length - 1)
    render()
    place()
  }

  function render() {
    popup.replaceChildren(
      ...items.map((item, i) => {
        const li = document.createElement('li')
        li.setAttribute('role', 'option')
        li.setAttribute('aria-selected', String(i === active))
        if (item.create) {
          li.className = 'create'
          li.textContent = `+ Нова стаття «${item.insert}»`
        } else {
          const name = document.createElement('span')
          name.textContent = item.insert
          li.append(name)
          if (item.alias) {
            const to = document.createElement('small')
            to.textContent = `→ ${item.title}`
            li.append(to)
          }
        }
        // mousedown, not click: keep focus in the field.
        li.addEventListener('mousedown', (e) => {
          e.preventDefault()
          accept(item)
        })
        return li
      }),
    )
    if (!popup.isConnected) document.body.append(popup)
  }

  function place() {
    const caret = caretRect(node, start)
    const width = 280
    popup.style.left = `${Math.max(8, Math.min(caret.left, innerWidth - width - 8))}px`
    const below = caret.top + caret.height + 4
    const height = popup.offsetHeight
    popup.style.top = `${below + height > innerHeight - 8 ? caret.top - height - 4 : below}px`
  }

  function accept(item: Item) {
    const caret = node.selectionStart ?? start
    const closed = node.value.slice(caret).startsWith(']]')
    node.setRangeText(item.insert + (closed ? '' : ']]'), start, caret, 'end')
    const end = start + item.insert.length + 2
    node.setSelectionRange(end, end)
    node.dispatchEvent(new Event('input', { bubbles: true })) // updates bind:value
    close()
  }

  function close() {
    popup.remove()
    items = []
    active = 0
  }

  function onKeydown(e: KeyboardEvent) {
    if (!popup.isConnected) return
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      active = (active + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length
      render()
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      accept(items[active])
    } else if (e.key === 'Escape') {
      close()
    } else {
      return
    }
    e.preventDefault()
    e.stopPropagation()
  }

  const onBlur = () => close()
  const onSelect = () => popup.isConnected && update()
  const el: HTMLElement = node // the union type confuses addEventListener's overloads
  el.addEventListener('input', update)
  el.addEventListener('keydown', onKeydown)
  el.addEventListener('blur', onBlur)
  el.addEventListener('click', onSelect)
  el.addEventListener('scroll', close)

  return {
    update(next: () => Summary[]) {
      getPages = next
    },
    destroy() {
      close()
      el.removeEventListener('input', update)
      el.removeEventListener('keydown', onKeydown)
      el.removeEventListener('blur', onBlur)
      el.removeEventListener('click', onSelect)
      el.removeEventListener('scroll', close)
    },
  }
}
