import { api, type Change, type TreeNode } from './api'

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

// The state of each article against the last saved version, by the path
// its tree node has (the file's path without .md).
export function articleStates(list: Change[]) {
  const out = new Map<string, Change['state']>()
  for (const c of list) if (/\.md$/i.test(c.path)) out.set(c.path.slice(0, -3), c.state)
  return out
}

// The tree with deleted articles put back where they were, as ghosts, so
// what is about to leave the world shows until the next version is saved.
// Folders that went with them come back as ghosts too.
export function withDeleted(tree: TreeNode[], states: Map<string, Change['state']>): TreeNode[] {
  const deleted = [...states].filter(([, s]) => s === 'deleted').map(([p]) => p)
  if (!deleted.length) return tree
  const root = $state.snapshot(tree) as TreeNode[] // a plain copy to add to
  for (const path of deleted.sort()) {
    let level = root
    const parts = path.split('/')
    let at = ''
    for (const [i, name] of parts.entries()) {
      at = at ? `${at}/${name}` : name
      let node = level.find((n) => n.path === at)
      if (!node) {
        node = { name, path: at, ghost: true }
        level.splice(ghostPlace(level, name, i < parts.length - 1), 0, node)
      } else if (i === parts.length - 1) {
        break // something lives there again (a folder of its children)
      }
      if (i < parts.length - 1) level = node.children ??= []
    }
  }
  return root
}

// Where a ghost goes among its siblings, which come groups (folders and
// articles holding others) first, each kind by name.
function ghostPlace(level: TreeNode[], name: string, group: boolean) {
  const isGroup = (n: TreeNode) => !n.title || !!n.children?.length
  const i = level.findIndex((n) =>
    isGroup(n) === group ? n.name.localeCompare(name, 'uk') > 0 : group && !isGroup(n),
  )
  return i < 0 ? level.length : i
}
