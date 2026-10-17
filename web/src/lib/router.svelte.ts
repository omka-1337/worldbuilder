// A minimal History API router: the URL is the state.

export const router = $state({
  path: location.pathname,
  query: new URLSearchParams(location.search),
})

function sync() {
  router.path = location.pathname
  router.query = new URLSearchParams(location.search)
}

addEventListener('popstate', sync)

export function navigate(href: string, { replace = false } = {}) {
  const u = new URL(href, location.href)
  history[replace ? 'replaceState' : 'pushState'](null, '', u.pathname + u.search + u.hash)
  sync()
  if (!u.hash) scrollTo(0, 0)
}

// Turns a URL path segment back into a title: /wiki/Сірий_Вовк → "Сірий Вовк".
export const titleFrom = (segment: string) => decodeURIComponent(segment).replaceAll('_', ' ')

// Routes same-origin link clicks through navigate() instead of a full reload.
export function interceptLinks(e: MouseEvent) {
  if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
  const a = (e.target as Element).closest('a')
  const href = a?.getAttribute('href')
  if (!a || !href || href.startsWith('#') || a.target || a.hasAttribute('download')) return
  const u = new URL(a.href)
  if (u.origin !== location.origin || /^\/w\/[^/]+\/files\//.test(u.pathname)) return
  e.preventDefault()
  navigate(u.pathname + u.search + u.hash)
}
