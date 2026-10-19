import { api, currentWorld } from './api'
import { refresh } from './store.svelte'

// The assistant's chat: the author's Claude Code, run by the server in the
// world's folder (internal/agent). A message's answer streams in as text
// and the steps it takes (reading, searching, editing articles), in order.

export type Part = { kind: 'text'; text: string; html?: string } | { kind: 'tool'; tool: string; target: string }

export interface ChatMessage {
  role: 'user' | 'assistant'
  parts: Part[]
  error?: string
  article?: string // the article open when the author wrote it
}

export interface AgentStatus {
  available: boolean
  version?: string
  rulesFile: string
}

export const chat = $state({
  messages: [] as ChatMessage[],
  session: '', // Claude Code's conversation, continued by each message
  running: false,
  status: null as AgentStatus | null,
})

// --- the conversation survives reloads, per world ------------------------

const storageKey = () => `worldbuilder:agent:${currentWorld()}`
let loadedFor = ''

export function loadChat() {
  const world = currentWorld()
  if (loadedFor === world) return
  loadedFor = world
  chat.messages = []
  chat.session = ''
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey()) ?? 'null')
    if (saved) {
      chat.messages = saved.messages ?? []
      chat.session = saved.session ?? ''
    }
  } catch {
    // Nothing saved, or storage unavailable: a new chat.
  }
  api.agentStatus().then(
    (s) => (chat.status = s),
    () => (chat.status = { available: false, rulesFile: '' }),
  )
}

function save() {
  try {
    localStorage.setItem(storageKey(), JSON.stringify({ session: chat.session, messages: chat.messages.slice(-60) }))
  } catch {
    // Storage full or unavailable: the chat lasts for this visit.
  }
}

export function newChat() {
  stop()
  chat.messages = []
  chat.session = ''
  save()
}

// --- sending --------------------------------------------------------------

let controller: AbortController | null = null

export function stop() {
  controller?.abort()
  controller = null
}

export async function send(text: string, article?: string) {
  if (!text.trim() || chat.running) return
  chat.messages.push({ role: 'user', parts: [{ kind: 'text', text }], article })
  chat.messages.push({ role: 'assistant', parts: [] })
  const answer = chat.messages[chat.messages.length - 1]
  chat.running = true
  controller = new AbortController()

  const addText = (t: string) => {
    const last = answer.parts.at(-1)
    if (last?.kind === 'text') last.text += t
    else answer.parts.push({ kind: 'text', text: t })
  }

  try {
    const res = await api.agentChat(text, chat.session, article, controller.signal)
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    for (;;) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      let nl: number
      while ((nl = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, nl).trim()
        buffer = buffer.slice(nl + 1)
        if (!line) continue
        const e = JSON.parse(line)
        if (e.session) chat.session = e.session
        if (e.type === 'text') addText(e.text)
        else if (e.type === 'tool') answer.parts.push({ kind: 'tool', tool: e.tool, target: e.target ?? '' })
        else if (e.type === 'error') answer.error = e.error === 'limit' ? 'limit' : e.error
      }
    }
  } catch (e) {
    if ((e as Error).name !== 'AbortError') answer.error = e instanceof Error ? e.message : String(e)
    else answer.error = 'stopped'
  } finally {
    chat.running = false
    controller = null
  }

  // The answer's text as the wiki shows Markdown, links to articles included.
  await Promise.all(
    answer.parts.map(async (p) => {
      if (p.kind === 'text' && p.text.trim()) p.html = await api.preview(p.text).catch(() => undefined)
    }),
  )
  save()
  // Articles may have changed.
  await refresh()
}
