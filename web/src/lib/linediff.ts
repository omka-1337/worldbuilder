// Which lines of a text are new against an earlier version, for marking
// changes in the editor's margin. A changed line counts as new.

export interface LineChanges {
  added: Set<number> // indexes of lines in the new text
  deleted: Set<number> // positions in the new text where old lines were removed (before that line; length = at the end)
  addedCount: number
  deletedCount: number
}

// Past this many cells the comparison of the changed middle is skipped and
// all of it counts as new: still right at the edges, and never slow.
const MAX_CELLS = 4_000_000

export function lineChanges(before: string, after: string): LineChanges {
  const a = before === '' ? [] : before.split('\n')
  const b = after.split('\n')
  const out: LineChanges = { added: new Set(), deleted: new Set(), addedCount: 0, deletedCount: 0 }

  // Common lines at the start and the end need no comparison.
  let start = 0
  while (start < a.length && start < b.length && a[start] === b[start]) start++
  let endA = a.length
  let endB = b.length
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) endA--, endB--

  const n = endA - start
  const m = endB - start
  const add = (j: number) => (out.added.add(j), out.addedCount++)
  const del = (at: number) => (out.deleted.add(at), out.deletedCount++)

  if (n * m > MAX_CELLS) {
    for (let j = start; j < endB; j++) add(j)
    for (let i = 0; i < n; i++) del(endB)
    return out
  }

  // Longest common subsequence of the middle, filled from the end so the
  // walk below can go forwards.
  const w = m + 1
  const lcs = new Uint32Array((n + 1) * w)
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i * w + j] =
        a[start + i] === b[start + j] ? lcs[(i + 1) * w + j + 1] + 1 : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1])
    }
  }
  let i = 0
  let j = 0
  while (i < n || j < m) {
    if (i < n && j < m && a[start + i] === b[start + j]) {
      i++, j++
    } else if (j < m && (i === n || lcs[i * w + j + 1] >= lcs[(i + 1) * w + j])) {
      add(start + j++)
    } else {
      del(start + j)
      i++
    }
  }
  // A line replaced by another shows as changed, not also as a removal.
  for (const at of out.deleted) if (out.added.has(at) || out.added.has(at - 1)) out.deleted.delete(at)
  return out
}
