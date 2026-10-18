import { api, type Summary, type TreeNode, type World } from './api'

// Data shared across views. Call refresh() after anything that changes the
// set of pages.
export const store = $state({
  world: null as World | null,
  pages: [] as Summary[],
  tree: [] as TreeNode[],
  version: 0, // bumped on every refresh, for views that show derived data
})

export function reset() {
  store.world = null
  store.pages = []
  store.tree = []
}

export async function refresh() {
  const [world, pages, tree] = await Promise.all([api.world(), api.pages(), api.tree()])
  store.world = world
  store.pages = pages
  store.tree = tree
  store.version++
}

export const typeLabel = (type?: string) => (type && store.world?.types[type]?.label) || ''
