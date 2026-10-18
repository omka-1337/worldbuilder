import { api, type Change } from './api'

// What is not saved as a version yet. The ribbon keeps it fresh; the
// versions page reads it too.
export const changes = $state({
  list: [] as Change[],
  ready: false, // false until the first answer, or while history is unavailable
})

export async function refreshChanges() {
  try {
    changes.list = await api.changes()
    changes.ready = true
  } catch {
    changes.ready = false
  }
}

// How a changed file is named to the author: an article by its title,
// anything else by its file name.
export function changeName(c: Change) {
  if (c.title) return c.title
  if (c.path === 'world.yaml') return 'Налаштування світу'
  const file = c.path.split('/').at(-1) ?? c.path
  return /\.md$/i.test(file) ? file.slice(0, -3) : file
}

// A description for a version, made from its changes: "Змінено «А», «Б»;
// додано «В»" — at most three names in all, then a count of the rest.
export function describe(list: Change[]) {
  const verbs: [Change['state'], string][] = [
    ['modified', 'змінено'],
    ['added', 'додано'],
    ['deleted', 'видалено'],
  ]
  let named = 0
  const parts = []
  for (const [state, verb] of verbs) {
    const names = list.filter((c) => c.state === state).map(changeName)
    if (!names.length) continue
    const shown = names.slice(0, Math.max(3 - named, 1))
    named += shown.length
    const rest = names.length - shown.length
    parts.push(`${verb} ${shown.map((n) => `«${n}»`).join(', ')}${rest ? ` і ще ${rest}` : ''}`)
  }
  const text = parts.join('; ')
  return text.charAt(0).toUpperCase() + text.slice(1)
}
