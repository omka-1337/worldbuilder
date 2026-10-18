import type { TreeNode } from './api'

// The article being dragged in the sidebar tree, shared by all tree levels.
export const drag = $state({
  title: '',
  over: '', // the row the pointer is over, if it accepts the drop
  inside: new Set<string>(), // the dragged article and everything nested in it
})

// Chrome cancels a drag whose dragstart changes the layout under the
// pointer, so the state (which restyles the tree) is set a moment later; a
// drag that ends before then must not leave it behind.
let pending: ReturnType<typeof setTimeout> | undefined

export function startDrag(node: TreeNode) {
  clearTimeout(pending)
  pending = setTimeout(() => begin(node))
}

function begin(node: TreeNode) {
  const inside = new Set<string>()
  const walk = (n: TreeNode) => {
    if (n.title) inside.add(n.title)
    n.children?.forEach(walk)
  }
  walk(node)
  drag.title = node.title ?? ''
  drag.inside = inside
}

export function endDrag() {
  clearTimeout(pending)
  drag.title = ''
  drag.over = ''
  drag.inside = new Set()
}

// An article accepts a drop unless it is the dragged one or inside it.
export const accepts = (node: TreeNode) => !!drag.title && !!node.title && !drag.inside.has(node.title)
