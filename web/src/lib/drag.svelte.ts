import type { TreeNode } from './api'

// The article being dragged in the sidebar tree, shared by all tree levels.
export const drag = $state({
  title: '',
  path: '', // the dragged article's own folder: nothing may go inside it
  over: '', // path of the row under the pointer, if it accepts the drop
})

// Chrome cancels a drag whose dragstart changes the layout under the
// pointer, so the state (which restyles the tree) is set a moment later; a
// drag that ends before then must not leave it behind.
let pending: ReturnType<typeof setTimeout> | undefined

export function startDrag(node: TreeNode) {
  clearTimeout(pending)
  pending = setTimeout(() => {
    drag.title = node.title ?? ''
    drag.path = node.path
  })
}

export function endDrag() {
  clearTimeout(pending)
  drag.title = ''
  drag.path = ''
  drag.over = ''
}

// Any article or folder accepts a drop, except the dragged article and
// everything nested in it.
export const accepts = (node: TreeNode) =>
  !!drag.title && node.path !== drag.path && !node.path.startsWith(drag.path + '/')
