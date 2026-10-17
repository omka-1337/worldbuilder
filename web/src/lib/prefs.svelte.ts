// Interface preferences, kept in localStorage: they belong to this browser,
// not to a world.

export type Theme = 'system' | 'light' | 'dark'
export type Side = 'left' | 'right'

const KEY = 'worldbuilder:prefs'

function load(): { theme: Theme; ribbon: Side } {
  const defaults = { theme: 'system' as Theme, ribbon: 'left' as Side }
  try {
    return { ...defaults, ...JSON.parse(localStorage.getItem(KEY) ?? '{}') }
  } catch {
    return defaults
  }
}

export const prefs = $state(load())

$effect.root(() => {
  $effect(() => {
    const { theme, ribbon } = prefs
    try {
      localStorage.setItem(KEY, JSON.stringify({ theme, ribbon }))
    } catch {
      // Private mode or storage disabled: preferences last for this session.
    }
    if (theme === 'system') delete document.documentElement.dataset.theme
    else document.documentElement.dataset.theme = theme
  })
})
