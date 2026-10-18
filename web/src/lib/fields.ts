// Names for infobox fields that are not in the article's type, as the
// server's World.fieldLabel gives them on the article page.

import type { World } from './api'

// One taken out of the type shows under its old label, one some other type
// has under that type's label. Anything else is the article's own field and
// its key is the name the author typed, unless it reads like a generated key
// ("дата-народження"), which is turned back into words.
export function fieldLabel(world: World | null | undefined, type: string, key: string): string {
  const types = world?.types ?? {}
  const former = types[type]?.former?.[key]
  if (former) return former
  for (const k of Object.keys(types).sort()) {
    const f = types[k].fields.find((f) => f.key === key && f.type !== 'section')
    if (f) return f.label
  }
  if (!key || !/^[\p{L}\p{N}]+(-[\p{L}\p{N}]+)*$/u.test(key) || key !== key.toLowerCase()) return key
  const words = key.replaceAll('-', ' ')
  return words[0].toUpperCase() + words.slice(1)
}
