// Small modal questions — a name to type, a yes/no — shown by the one
// <Dialogs /> in the app, in place of the browser's prompt() and confirm().

type Request = {
  title: string
  text?: string
  value?: string // present: ask for a name; absent: ask for confirmation
  ok: string
  danger?: boolean
  resolve: (answer: string | boolean | null) => void
}

export const dialog = $state({ current: null as Request | null })

function open(r: Omit<Request, 'resolve'>) {
  return new Promise<string | boolean | null>((resolve) => {
    dialog.current?.resolve(null)
    dialog.current = { ...r, resolve }
  })
}

// Resolves to the trimmed text, or null when cancelled or left empty.
export async function askName(title: string, value = '', ok = 'Готово') {
  const answer = await open({ title, value, ok })
  return typeof answer === 'string' && answer.trim() ? answer.trim() : null
}

export async function confirmAction(title: string, text: string, ok: string, danger = false) {
  return (await open({ title, text, ok, danger })) === true
}

export function answer(a: string | boolean | null) {
  const r = dialog.current
  dialog.current = null
  r?.resolve(a)
}
