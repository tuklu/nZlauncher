# Contributing

## Building

```sh
# Arch
sudo pacman -S go gtk4 gtk4-layer-shell
# Debian/Ubuntu (gtk4-layer-shell is not packaged on 24.04 — see ci.yml
# for the from-source recipe)
sudo apt install golang libgtk-4-dev libgirepository1.0-dev

go build -o nz .
```

The first build compiles the gotk4 GTK bindings and takes several
minutes; afterwards Go's build cache makes rebuilds near-instant.

## How it works

One engine, three consumers:

```
keystroke ──► queryActions(raw)              query.go
                ├─ "" ............ top items by frecency + context
                ├─ group name .... that group
                ├─ NL verb ....... nl.go routes to a provider
                ├─ prefix word ... built-in provider, or providers.d plugin
                └─ otherwise ..... rank(catalog + apps + windows) + fallbacks

consumers: tui.go (bubbletea), gtk.go (gotk4 + layer-shell),
           runQuery TSV (`nz query`, scripts)
```

Every pick calls `recordLaunch(id, query)` which appends one line to
`events.jsonl` (timestamp, typed query, battery, SSID, focused app).
The log is replayed on each query to build frecency stats and the
context model — there is no daemon and no state between launches.

### File map

| File | Owns |
|---|---|
| `query.go` | engine entry, provider registry, ranking |
| `match.go` | match scoring (name/word/substring/fuzzy/keywords) |
| `context_rank.go` | learned model: hour affinity, Markov successors, env signals, aliases |
| `frecency.go` / `store.go` | scoring + event log |
| `nl.go` | natural-language verb grammar |
| `catalog.go` | catalog/settings parsing, embedded defaults |
| `apps.go` | .desktop scanning, category icons/keywords |
| `providers.go`, `system_providers.go`, `wifi.go`, `monitor.go`, `clipboard.go`, `emoji.go`, `keys.go`, `files.go`, `transcode.go` | built-in providers |
| `plugins.go` | providers.d external provider contract |
| `fallback.go`, `expr.go` | no-dead-end intents, calculator |
| `tui.go`, `theme.go` | bubbletea face + nz-tui.toml theming |
| `gtk.go` | GTK4 layer-shell face + nz-gtk.css theming |
| `pick.go` | `nz pick` dmenu utility |
| `exec.go`, `main.go` | detached launch, CLI dispatch |

### Adding a built-in provider

A provider is one function and one registration line:

```go
// yourthing.go
func yourThingActions(q string) []Action {
    q = strings.ToLower(q) // you receive raw case; lowercase for matching
    var acts []Action
    // ... build entries: ID (learning identity), Name, Icon, Group, Cmd
    return acts
}
```

```go
// query.go, prefixProviders table:
{[]string{"yourthing", "yt"}, yourThingActions},
```

Conventions: return `nil` when a required tool is missing (the provider
just disappears), keep IDs stable so learning sticks, cap result counts,
end with a `tuiTail(...)` escape hatch if a full TUI manager exists for
the domain.

If it doesn't need to ship in the binary, prefer a `providers.d` plugin
instead — any executable printing `id<TAB>cmd<TAB>display` lines.

### Things that bite

- The TSV protocol breaks on raw tabs/newlines — output is sanitized in
  `runQuery`, but keep Cmd strings tab-free (see clipboard.go's bare-id
  decode for the pattern).
- `hyprctl` cannot dispatch `__lua` binds (Lua-config Hyprland) — the
  keys provider marks those view-only.
- GTK: the Wayland app_id is the GtkApplication id (`dev.tuklu.nz`);
  the SearchEntry consumes Escape unless your controller uses the
  capture phase; settings readers must lazy-load the catalog.

## Workflow

`dev` is the default working branch; `main` is stable.

- PRs target `dev`; CI (vet + build) runs on every push and PR
- conventional commits: `feat:`, `fix:`, `chore:`, `refactor:`, `docs:`
- release: merge `dev` → `main`, tag `vX.Y.Z`, push — Actions builds the
  prebuilt on Ubuntu (broad glibc floor) and attaches it to the release

```sh
git checkout main && git merge dev
git tag v0.2.0 && git push origin main v0.2.0
```

## Style

- Go: functional over OOP, stdlib first, table-driven where it fits
- dependencies are a big deal — the engine is BurntSushi/toml plus the
  two face libraries (charmbracelet, gotk4) and nothing else; propose
  before adding
- self-documenting names; comments only for non-obvious decisions
- no emojis in code or output (nerd-font glyphs are fine)
