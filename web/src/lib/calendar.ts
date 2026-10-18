// Counting time in a world: epochs, years and parts of a year, as set on
// the Calendar page (world.yaml `calendar`).

import type { TimelineEntry, TimelineYear, World } from './api'

const ROMAN: [number, string][] = [[100, 'C'], [90, 'XC'], [50, 'L'], [40, 'XL'], [10, 'X'], [9, 'IX'], [5, 'V'], [4, 'IV'], [1, 'I']]

export function roman(n: number) {
  let out = ''
  for (const [v, s] of ROMAN) while (n >= v) (out += s), (n -= v)
  return out
}

// "III Епохи", or the name the author gave the epoch.
export function epochName(world: World | null | undefined, epoch: number) {
  if (!epoch) return ''
  const own = world?.calendar?.epochs?.[epoch - 1]?.name
  return own || `${roman(epoch)}${world?.era ? ' ' + world.era : ''}`
}

// "139 III Епохи", or "343 ЕП" in a world without epochs.
export function yearLabel(world: World | null | undefined, epoch: number, year: number) {
  if (epoch) return `${year} ${epochName(world, epoch)}`
  return `${year}${world?.era ? ' ' + world.era : ''}`
}

// Where each epoch starts on one shared axis, in years. Only possible when
// every epoch before the last one in use has a known length; otherwise null,
// and each epoch keeps an axis of its own.
export function epochOffsets(world: World | null | undefined, inUse: number[]): Map<number, number> | null {
  if (inUse.length < 2 || inUse.some((e) => e < 1)) return null
  const last = Math.max(...inUse)
  const out = new Map<number, number>()
  let at = 0
  for (let e = 1; e <= last; e++) {
    out.set(e, at)
    if (e < last) {
      const years = world?.calendar?.epochs?.[e - 1]?.years
      if (!years) return null
      at += years
    }
  }
  return out
}

// A part of the year as a fraction of it, when the calendar says how many
// parts a year has; a part beyond that count sits at the year's end.
export function yearFraction(world: World | null | undefined, part = 0) {
  const parts = world?.calendar?.yearParts
  if (!parts || !part) return 0
  return Math.min(part, parts) / (parts + 1)
}

export interface Moment {
  epoch: number
  year: number
  part: number
}

const later = (a: Moment, b: Moment) =>
  a.epoch !== b.epoch ? a.epoch > b.epoch : a.year !== b.year ? a.year > b.year : a.part > b.part

// The world's present: the latest date anywhere on its timeline.
export function presentMoment(years: TimelineYear[]): Moment | null {
  let now: Moment | null = null
  for (const y of years)
    for (const e of y.entries) {
      const points: Moment[] = [{ epoch: e.epoch ?? 0, year: e.year, part: e.part ?? 0 }]
      if (e.endYear) points.push({ epoch: e.endEpoch ?? e.epoch ?? 0, year: e.endYear, part: e.endPart ?? 0 })
      for (const p of points) if (!now || later(p, now)) now = p
    }
  return now
}

// Dates whose part of the year is beyond what the calendar allows.
export function outOfCalendar(world: World | null | undefined, years: TimelineYear[]): TimelineEntry[] {
  const parts = world?.calendar?.yearParts
  if (!parts) return []
  return years.flatMap((y) => y.entries.filter((e) => (e.part ?? 0) > parts || (e.endPart ?? 0) > parts))
}

// An example date in this world's own notation, for hints: "139.050 III Епохи".
export function exampleDate(world: World | null | undefined) {
  const parts = world?.calendar?.yearParts
  const part = parts ? '.' + String(Math.ceil(parts / 2)).padStart(String(parts).length, '0') : ''
  const epochs = world?.calendar?.epochs?.length ?? 0
  return yearLabel(world, epochs, 139).replace(/^139/, '139' + part)
}
