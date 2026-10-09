# Contributing

## Building

```sh
# Arch
sudo pacman -S go gtk4 gtk4-layer-shell
# Debian/Ubuntu (gtk4-layer-shell is not packaged on 24.04 — see ci.yml
# for the from-source recipe)
sudo apt install golang libgtk-4-dev libgirepository1.0-dev

go build -o nz ./cmd/nz
```

The first build compiles the gotk4 GTK bindings and takes several
minutes; afterwards Go's build cache makes rebuilds near-instant.

## How it works

One engine, three consumers:

```
keystroke ──► engine.Query(raw)              internal/engine/query.go
                ├─ "" ............ top items by frecency + context
                ├─ group name .... that group
                ├─ NL verb ....... nl.go routes to a provider
                ├─ prefix word ... providers.ByPrefix: built-in or providers.d plugin
                └─ otherwise ..... rank(catalog + apps + windows) + fallbacks

consumers: internal/ui/tui (bubbletea), internal/ui/gtk (gotk4 + layer-shell),
           `nz query` TSV (scripts)
```

Every pick calls `learn.Record(id, query)`, which appends one line to
`events.jsonl` (timestamp, typed query, battery, SSID, focused app).
The log is replayed on each query to build frecency stats and the
context model. There is no daemon and no state between launches.

The picked command then runs through `shell.LaunchAfterClose`: a
detached `nz after-close` waits for the launcher surface to leave
Hyprland before running it, so captures and key-sending actions never
see the launcher.

### Layout

```
cmd/nz/                 CLI dispatch, `bar` toggle, `pick` dmenu utility
internal/
  action/               the Action type every layer passes around
  engine/               Query, match scoring, ranking, NL grammar, fallbacks, calculator
  providers/            apps, windows, keys, wifi, bluetooth, audio, files, clipboard,
                        emoji, monitor, transcode, providers.d plugins, prefix registry
  learn/                event log, frecency, context model (hour, successors, env, aliases)
  config/               catalog.toml + settings, embedded defaults
  icons/                TUI icon library loader, embedded nerd.toml
  shell/                quoting, detached launch, after-close handoff
  sysinfo/              battery and SSID probes for event context
  ui/tui/               bubbletea face + nz-tui.toml theming
  ui/gtk/               GTK4 face + nz-gtk.css theming
```

Imports only point down this list: `providers` never imports `engine`,
so the engine can call providers by name (the NL layer does) without a
cycle. Embedded defaults live next to the package that embeds them,
because `go:embed` can't reach a parent directory.

### Adding a built-in provider

A provider is one function and one registration line:

```go
// internal/providers/yourthing.go
func YourThing(q string) []action.Action {
    q = strings.ToLower(q) // you receive raw case; lowercase for matching
    var acts []action.Action
    // ... build entries: ID (learning identity), Name, Icon, Group, Cmd
    return acts
}
```

```go
// internal/providers/registry.go, prefixed table:
{[]string{"yourthing", "yt"}, YourThing},
```

Conventions: return `nil` when a required tool is missing (the provider
just disappears), keep IDs stable so learning sticks, cap result counts,
end with a `tuiTail(...)` escape hatch if a full TUI manager exists for
the domain. Quote anything user-controlled with `shell.Quote`.

If it doesn't need to ship in the binary, prefer a `providers.d` plugin
instead — any executable printing `id<TAB>cmd<TAB>display` lines.

### Things that bite

- The TSV protocol breaks on raw tabs/newlines — output is sanitized in
  `cmd/nz`'s `runQuery`, but keep Cmd strings tab-free (see
  clipboard.go's bare-id decode for the pattern).
- `hyprctl` cannot dispatch `__lua` binds (Lua-config Hyprland), so
  `providers/keys.go` recovers them from `hl.bind(...)` calls in the
  config source and runs them through `hyprctl eval`.
- GTK: the Wayland app_id is the GtkApplication id (`dev.tuklu.nz`);
  the SearchEntry consumes Escape unless your controller uses the
  capture phase.

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
