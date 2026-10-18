export interface World {
  name: string
  description?: string
  mainPage: string
  era?: string
  language: string
  types: Record<string, TypeDef>
}

export interface FieldDef {
  key: string // empty until saved; then fixed, so renaming keeps articles working
  label: string
  type?: string // "date" or empty for text
}

export interface TypeDef {
  label: string
  color?: string // "#rrggbb"
  event?: boolean
  fields: FieldDef[]
}

export interface TypeInfo extends TypeDef {
  key: string
  count: number
}

export interface WorldInfo {
  id: string
  name: string
  description?: string
  pages: number
  modified: string
}

export interface Summary {
  title: string
  type?: string
  aliases: string[]
  categories: string[]
}

export interface Heading {
  level: number
  text: string
  id: string
}

export interface PageView {
  title: string
  path: string
  type?: string
  typeLabel?: string
  redirectedFrom?: string
  aliases: string[]
  categories: string[]
  infobox: { label: string; html: string }[]
  image?: string
  html: string
  headings: Heading[]
  backlinks: string[]
  modified: string
}

// An article as the editor form sees it; the server turns it into the file.
export interface Source {
  exists: boolean
  create?: boolean // on save: fail rather than overwrite an existing article
  title: string // on save: a different title renames the article
  type: string
  aliases: string[]
  categories: string[]
  infobox: { key: string; value: string }[]
  body: string
}

export interface Hit {
  title: string
  snippet: string
}

export interface Category {
  name: string
  count: number
}

export interface Wanted {
  title: string
  from: string[]
}

export interface TimelineEntry {
  title: string
  type?: string
  typeLabel?: string
  field: string
  year: number
  endYear?: number
  text: number
  event: boolean
}

export interface TimelineYear {
  year: number
  main: TimelineEntry
  entries: TimelineEntry[]
}

export class HttpError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
  }
}
export class NotFoundError extends HttpError {}

async function check(r: Response): Promise<Response> {
  if (r.status === 404) throw new NotFoundError(r.url, 404)
  if (!r.ok) throw new HttpError((await r.text()).trim() || r.statusText, r.status)
  return r
}

async function get<T>(url: string): Promise<T> {
  return (await check(await fetch(url))).json()
}

const enc = encodeURIComponent

// The world every call and link below refers to. WorldShell sets it before
// rendering anything inside a world, and remounts on switching worlds, so it
// does not need to be reactive.
let world = ''
export const setWorld = (id: string) => (world = id)
const w = () => `/api/worlds/${enc(world)}`

async function send<T>(method: string, url: string, body: unknown): Promise<T> {
  const r = await check(
    await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  )
  return r.json()
}

export const api = {
  worlds: () => get<{ root: string; worlds: WorldInfo[] }>('/api/worlds'),
  createWorld: (name: string) => send<WorldInfo>('POST', '/api/worlds', { name }),

  world: () => get<World>(w()),
  pages: () => get<Summary[]>(`${w()}/pages`),
  page: (title: string) => get<PageView>(`${w()}/pages/${enc(title)}`),
  search: (q: string, limit = 50) => get<Hit[]>(`${w()}/search?q=${enc(q)}&limit=${limit}`),
  categories: () => get<Category[]>(`${w()}/categories`),
  category: (name: string) => get<Summary[]>(`${w()}/categories/${enc(name)}`),
  wanted: () => get<Wanted[]>(`${w()}/wanted`),
  timeline: () => get<TimelineYear[]>(`${w()}/timeline`),
  types: () => get<TypeInfo[]>(`${w()}/types`),
  saveTypes: (types: TypeInfo[]) => send<TypeInfo[]>('PUT', `${w()}/types`, types),

  source: (title: string) => get<Source>(`${w()}/source/${enc(title)}`),
  // Resolves to the title the page ended up with.
  saveSource: async (title: string, source: Omit<Source, 'exists'>) =>
    (await send<{ title: string }>('PUT', `${w()}/source/${enc(title)}`, source)).title,
}

// Same normalization as vault.Key on the server.
export const key = (title: string) =>
  title.replaceAll('_', ' ').trim().split(/\s+/).join(' ').toLowerCase()

const slugged = (title: string) => enc(title.trim().replaceAll(' ', '_'))

export const worldHref = (id = world, sub = '') => `/w/${enc(id)}${sub}`
export const pageHref = (title: string) => worldHref(world, '/wiki/' + slugged(title))
export const editHref = (title: string) => worldHref(world, '/edit/' + slugged(title))
export const categoryHref = (name: string) => worldHref(world, '/category/' + slugged(name))
