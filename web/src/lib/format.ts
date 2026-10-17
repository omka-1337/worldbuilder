// Ukrainian plural forms: plural(n, ['стаття', 'статті', 'статей']).
export function plural(n: number, [one, few, many]: [string, string, string]) {
  const form = new Intl.PluralRules('uk').select(n)
  return `${n} ${form === 'one' ? one : form === 'few' ? few : many}`
}

// Groups an already-sorted list by first letter, like wiki index pages.
export function byLetter<T>(items: T[], name: (item: T) => string): [string, T[]][] {
  const groups: [string, T[]][] = []
  for (const item of items) {
    const letter = (name(item)[0] ?? '#').toLocaleUpperCase('uk')
    const last = groups.at(-1)
    if (last && last[0] === letter) last[1].push(item)
    else groups.push([letter, [item]])
  }
  return groups
}

// Splits text around case-insensitive matches of q, for <mark> highlighting.
export function highlight(text: string, q: string) {
  const parts: { text: string; match: boolean }[] = []
  const lower = text.toLowerCase()
  const needle = q.trim().toLowerCase()
  let i = 0
  while (needle) {
    const j = lower.indexOf(needle, i)
    if (j < 0) break
    parts.push({ text: text.slice(i, j), match: false }, { text: text.slice(j, j + needle.length), match: true })
    i = j + needle.length
  }
  parts.push({ text: text.slice(i), match: false })
  return parts
}

// "5 хвилин тому", "вчора", "3 місяці тому".
export function ago(date: string | Date, lang = 'uk') {
  const seconds = (new Date(date).getTime() - Date.now()) / 1000
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' })
  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ['year', 31536000],
    ['month', 2592000],
    ['week', 604800],
    ['day', 86400],
    ['hour', 3600],
    ['minute', 60],
  ]
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) return rtf.format(Math.round(seconds / size), unit)
  }
  return rtf.format(0, 'minute')
}
