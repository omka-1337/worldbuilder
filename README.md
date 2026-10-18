# Worldbuilder

A local wiki for worldbuilding. Articles are structured like Wikipedia's —
infoboxes, categories, redirects, "what links here", wanted pages — on top of
plain Markdown files, plus a timeline built from the dates in those articles.

## Running

```sh
make              # builds the UI and the ./worldbuilder binary
./worldbuilder    # opens the world picker in the browser
```

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
Types marked as events name the years they fall in. Type and field keys are generated once
from their names and never change, so renaming keeps articles working.

## Code layout

- `cmd/worldbuilder` — entry point
- `internal/library` — the library of worlds: listing, creating, opening
- `internal/vault` — one world: reading files, the link index, Markdown
  rendering, timeline, article types
- `internal/server` — JSON API and serving the UI
- `web` — the UI (Svelte), embedded into the binary with `go:embed`
