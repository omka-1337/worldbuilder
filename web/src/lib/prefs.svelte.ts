// Interface preferences, kept in localStorage: they belong to this browser,
// not to a world.

export type Theme = 'system' | 'light' | 'dark'
export type Side = 'left' | 'right'

const KEY = 'worldbuilder:prefs'

// The article tree's width in px, dragged by its edge.
export const SIDEBAR = { min: 200, max: 560, default: 272 }

function load(): {
  theme: Theme
  ribbon: Side
  sidebar: number
  sidebarHidden: boolean
  agentOpen: boolean
  hideAsides: boolean
} {
  const defaults = {
    theme: 'system' as Theme,
    ribbon: 'left' as Side,
    sidebar: SIDEBAR.default,
    sidebarHidden: false,
    agentOpen: false,
    hideAsides: false, // the author's [!author] asides hidden in articles, for a clean read
  }
  try {
    return { ...defaults, ...JSON.parse(localStorage.getItem(KEY) ?? '{}') }
  } catch {
    return defaults
  }
}

export const prefs = $state(load())

$effect.root(() => {
  $effect(() => {
    const { theme, ribbon, sidebar, sidebarHidden, agentOpen, hideAsides } = prefs
    try {
      localStorage.setItem(KEY, JSON.stringify({ theme, ribbon, sidebar, sidebarHidden, agentOpen, hideAsides }))
    } catch {
      // Private mode or storage disabled: preferences last for this session.
    }
    if (theme === 'system') delete document.documentElement.dataset.theme
    else document.documentElement.dataset.theme = theme
  })
})
