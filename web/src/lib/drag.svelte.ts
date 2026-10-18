import type { TreeNode } from './api'

// What is being dragged in the sidebar tree, shared by all tree levels: an
// article (by title) or a plain folder (by path).
export const drag = $state({
  title: '', // the dragged article
  folder: '', // the dragged plain folder
  path: '', // the dragged node's own folder: nothing may go inside it
  over: '', // path of the row under the pointer, if it accepts the drop
})

export const dragging = () => !!(drag.title || drag.folder)

// Chrome cancels a drag whose dragstart changes the layout under the
// pointer, so the state (which restyles the tree) is set a moment later; a
// drag that ends before then must not leave it behind.
let pending: ReturnType<typeof setTimeout> | undefined

export function startDrag(node: TreeNode) {
  clearTimeout(pending)
  pending = setTimeout(() => {
    drag.title = node.title ?? ''
    drag.folder = node.title ? '' : node.path
    drag.path = node.path
  })
}

export function endDrag() {
  clearTimeout(pending)
  drag.title = ''
  drag.folder = ''
  drag.path = ''
  drag.over = ''
}

// Any article or folder accepts a drop, except the dragged node and
// everything nested in it.
export const accepts = (node: TreeNode) =>
  dragging() && !node.ghost && node.path !== drag.path && !node.path.startsWith(drag.path + '/')
