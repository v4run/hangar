# Hangar v2 — alpha

A ground-up redesign of the Hangar TUI. See the parent repo's
`docs/plans/2026-10-18-tui-redesign.md` for the full design.

## Install

```
go install github.com/v4run/hangar/v2/cmd/hangar@latest
```

Overwrites the `hangar` binary on your `$PATH`. v2 reads your v1
config at `~/.hangar/config.yaml` on first run, migrates it in memory,
and writes the result to `~/.hangar/v2/config.yaml`. **Your v1
config is never touched.**

## Keymap

Three mode-selectors, no overloaded letters:

- `/` — filter the current list
- `:` — command bar (`:conn`, `:db`, `:group`, `:add`, `:import`,
  `:help`, …)
- `space` — leader menu that *shows* every next key and what it does

In browse mode:

| Key | Action |
| --- | --- |
| `j` / `k` / arrows | up / down |
| `h` / `l` | collapse / expand a group row |
| `enter` | connect / launch DB client / toggle group |
| `tab` / `shift+tab` | switch detail-pane tabs |
| `c` | connect to selected |
| `a` | add connection |
| `e` | edit selected (connection / database / group) |
| `d` | delete selected (connection / database) |
| `g` | new group |
| `i` | import ssh command |
| `o` | edit notes |
| `?` | help overlay |
| `q` / `ctrl+c` | quit |

Floating palettes (add/edit/host-key/import):

- `tab` / `shift+tab` move between fields; `enter` / `ctrl+s` saves
- Cursor movement, insert, delete, paste all work inline (bubbles
  textinput — no nav↔edit toggle)
- `esc` cancels

## Multi-level groups

A group has an optional parent. Defaults (user, identity, SSH
options, shell init, scripts, env) cascade root → leaf → connection;
first non-empty value wins. The Details tab annotates inherited
rows with `— from prod/us-east/` so you always see *where* a value
came from.

Create nested groups by giving a slash-separated name:
`prod/us-east/web` — intermediate groups are created automatically.

## What's implemented

- Phase 1 — foundation (theme, help footer, proportional sidebar)
- Phase 2 — picker, `:` command bar, `space` leader
- Phase 3 — tabbed detail (Details / Scripts / Forwards / Notes),
  floating palettes, mode chip in status bar
- Phase 4 — multi-level group inheritance, v1 → v2 migrator,
  Config.Validate (cycles / orphans / sibling dupes),
  Config.Resolve with FieldScope/FieldOrigin for the detail
  pane's "— from" annotations

Features: connections (CRUD + connect), databases (CRUD + launch
psql/mysql/redis-cli/sqlite3), scripts (CRUD + run via picker),
notes, groups (CRUD + delete re-parents contents), host-key
confirm modal, SSH command import, port-forward view, mouse
clicks + wheel on sidebar.

## What's still rough

- Shell-init editor is single-line (paste multi-line, or edit via
  `$EDITOR` — the latter isn't wired yet). Will swap to
  `bubbles/textarea` in alpha.2.
- Tag editor is comma-separated text; the v1 token editor hasn't
  been ported yet.
- No port-forward manager actions (view only).
- No visual select / reorder / pin.

## Development

```
cd v2
go build ./...
go test ./...
go run ./cmd/hangar
```
