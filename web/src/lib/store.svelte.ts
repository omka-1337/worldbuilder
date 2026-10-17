import { api, type Summary, type World } from './api'

// Data shared across views. Call refresh() after anything that changes the
// set of pages.
export const store = $state({
  world: null as World | null,
  pages: [] as Summary[],
})

export function reset() {
  store.world = null
  store.pages = []
}

export async function refresh() {
  const [world, pages] = await Promise.all([api.world(), api.pages()])
  store.world = world
  store.pages = pages
}

export const typeLabel = (type?: string) => (type && store.world?.types[type]?.label) || ''
