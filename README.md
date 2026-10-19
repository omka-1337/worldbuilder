# Worldbuilder

A local wiki for worldbuilding. Articles are structured like Wikipedia's —
infoboxes, categories, redirects, "what links here", wanted pages — on top of
plain Markdown files, plus a timeline built from the dates in those articles.

## Running

```sh
make              # builds the UI and the ./worldbuilder binary
./worldbuilder    # opens the world picker in the browser
```

While it runs, a tray icon opens the app or the worlds folder and stops the
server; `-no-tray` turns it off (it is also off when there is no desktop
session, e.g. over SSH, and in cross-compiled macOS builds, which lack cgo).

Worlds live in `<Documents>/Worldbuilder/`: on Linux the documents folder
comes from `~/.config/user-dirs.dirs`, on Windows it is the Documents known
folder (OneDrive redirection included). Use `-library <path>` for another
location.

Development with UI hot reload: `make dev` (optionally `LIBRARY=<path>`), then
open http://localhost:5173. Release builds for Linux, macOS and Windows:
`make release`.

## World format

Each world is a folder named after it inside the library:

```
Documents/Worldbuilder/
├── My World/
│   ├── world.yaml
│   ├── Main page.md
│   └── …
└── Another World/
```

Every `.md` file in a world (outside hidden folders) is an article. The file
name is the title unless the frontmatter sets `title`. In the app, articles
are edited through a form — type, infobox fields, other names, categories —
next to the text; the frontmatter below is what the form writes. Typing
`[[` suggests article titles and aliases. Renaming an article moves its file
and keeps the old title as an alias, as Wikipedia does, so links keep
working. Files can also be edited in any editor; changes are picked up
automatically.

```markdown
---
type: character                   # article type → infobox fields
aliases: [Grey Wolf]              # redirects to this article
categories: [Characters, Mages]
infobox:
  born: 312, [[Valmor]]
  affiliation: "[[Order of Ash]]" # quote wikilinks inside YAML
  image: "![[aedrin.png]]"        # an image anywhere in the world folder
---
**Aedrin** founded the [[Order of Ash|Order]]. See [[#Legacy]].
```

Every article can have an infobox picture with a caption, whatever its type:
drop a file onto the editor, pick one or paste it. It is saved in the
article's folder under the article's name (`Valmor/Valmor.png`), so it moves
along with the article. PNG, JPEG, GIF and WebP are accepted; SVG is not, as
it can carry scripts, and files from a world are served sandboxed.

Articles can be grouped by dragging one onto another in the sidebar tree.
The dropped article moves into a folder named after the parent, next to the
parent's file (`War.md` + `War/Battle.md`); its own children move with it,
and renaming the parent renames the folder. Links are by title, so moving
never breaks them.

`world.yaml` holds the world's name, main page and article types. Types are
edited on the app's "Article types" page, so users never touch the YAML.
Each type has a color and lists its infobox fields; a field of kind
*date* puts the article on the timeline — a single year (`343`) as a dot, a
range (`340–347`) as a bar below the axis — drawn in the type's color.
Types marked as events name the years they fall in. Two date fields of a
type can form a period (born → died), drawn as one bar.

The "Calendar" page sets how time is counted, entirely in the author's
terms: a hierarchy of time units from the largest down (an era or epoch,
years, months, days — whatever the world has), each written as a number, a
Roman numeral or one of a list of names, with how many fit in the unit above
and exceptions for particular values (`I: 500` years, `Талець: 28` days);
and templates for how dates are written, such as
`{Рік}[.{Частина}][ р.] {Епоха} Епохи` (`[ … ]` is optional). A range is two
dates joined by a dash, and its first date may leave out what the second
says. The timeline counts in one unit (the step, usually the year); units
above it lay out one shared axis when their lengths are known, and separate
tabs otherwise. The latest date in the world counts as the present: open
periods run up to it. A world without a calendar counts plain years.
An article can mark its own moments in the text with date anchors,
`{{145.126 III Епохи | Проголошення ТАТ}}` (in the editor: select the date,
right-click, "Додати дату"). The article shows the date where it stands,
with the description on hover; a "Хронологія" tab lists the anchors in
time order and leads back to each.
Type and field keys are generated once
from their names and never change, so renaming keeps articles working.

Every world is also a Git repository, created with the world (or the first
time an older world is opened) and holding a first version of everything in
it; deleted articles in `.trash/` stay out of it. Git runs inside the app,
so nothing needs installing. The bottom of the tools panel shows how many
files have changed since the last saved version; the "Versions" page lists
them and saves a new version with a description, written by the author or
made from the changes. Versions are signed with the Git identity from the
user's global Git config when there is one.

The assistant (the sparkles button in the tools panel) is a chat beside the
page. It runs the author's own Claude Code, found on the machine and signed
in with their Claude account, in the world's folder: `claude -p` with only
the file tools (read, search, edit, write), so it can work on articles but
cannot run commands, browse or delete anything. It is told how this app's
files are written (`internal/agent/rules.go`): frontmatter and infobox keys
of the world's types, links, pictures, callouts, date anchors, and the
world's calendar with dates from its articles as examples. The article
open in the app goes along with each message. An author can add rules for
one world in `.worldbuilder/agent.md`. Every change it makes shows in the
versions and can be undone there.

## Code layout

- `cmd/worldbuilder` — entry point
- `internal/library` — the library of worlds: listing, creating, opening
- `internal/vault` — one world: reading files, the link index, Markdown
  rendering, timeline, article types
- `internal/agent` — the writing assistant: running Claude Code, its rules
- `internal/history` — a world's versions, kept in Git (go-git)
- `internal/server` — JSON API and serving the UI
- `web` — the UI (Svelte), embedded into the binary with `go:embed`
