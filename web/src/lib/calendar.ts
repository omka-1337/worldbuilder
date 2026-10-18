// Helpers around the world's calendar (world.yaml `calendar`, edited on the
// Calendar page). Reading dates and placing them happens on the server.

import type { Calendar, World } from './api'

// An example date in the world's own notation, for hints: the first format
// filled with sample values, optional parts included.
export function exampleDate(world: World | null | undefined) {
  const cal: Calendar | undefined = world?.calendar
  if (!cal?.units?.length || !cal.formats?.length) return `343${world?.era ? ' ' + world.era : ''}`
  return cal.formats[0]
    .replace(/\{([^}]*)\}/g, (_, name: string) => {
      const u = cal.units.find((x) => x.name.toLowerCase() === name.trim().toLowerCase())
      if (!u) return name
      if (u.write === 'roman') return 'III'
      if (u.write === 'names') return u.names?.[0] ?? name
      return u.per ? String(Math.ceil(u.per / 2)).padStart(String(u.per).length, '0') : '139'
    })
    .replace(/[[\]]/g, '')
}
