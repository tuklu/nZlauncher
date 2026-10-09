# Development

## Build

```sh
# Arch
sudo pacman -S go gtk4 gtk4-layer-shell

# Debian/Ubuntu. gtk4-layer-shell isn't packaged on 24.04,
# .github/workflows/ci.yml has the build-from-source steps.
sudo apt install golang libgtk-4-dev libgirepository1.0-dev

go build -o nz ./cmd/nz
go vet ./...
```

The first build compiles the gotk4 bindings and takes a few minutes.
After that the build cache makes it quick.

The cgo `conflicting types for built-in function 'free'` warnings come
from gotk4 and are harmless.

To try a change without touching your installed copy:

```sh
go build -o /tmp/nz ./cmd/nz
/tmp/nz query "turn off wifi"
/tmp/nz tui
```

`nz query` prints `id<TAB>cmd<TAB>display` for each result, which is
the easiest way to check the engine. Saving its output for a set of
queries before and after a change catches most regressions.

## Layout

```
cmd/nz/                 CLI dispatch, the bar toggle, nz pick
internal/
  action/               the Action type
  engine/               Query, match scoring, ranking, sentence grammar, fallbacks, calculator
  providers/            apps, windows, keys, wifi, bluetooth, audio, files, clipboard,
                        emoji, monitor, transcode, plugins, the prefix word table
  learn/                event log, frecency, context model
  config/               catalog.toml and settings
  icons/                TUI icon libraries
  shell/                quoting, detached launch, after-close
  sysinfo/              battery and wifi probes
  ui/tui/               bubbletea face and its theme
  ui/gtk/               GTK4 face
```

Imports only go down this list. `providers` never imports `engine`, so
the sentence grammar in `engine` can call providers by name without an
import cycle.

Default files that get embedded (`catalog.toml`, `nerd.toml`,
`nz-tui.toml`, `emoji.txt`) sit next to the package that embeds them.
`go:embed` can't reach a parent directory.

## Adding a provider

A provider is a function from the text after the prefix word to a list
of actions:

```go
// internal/providers/yourthing.go
package providers

func YourThing(q string) []action.Action {
	q = strings.ToLower(q) // q keeps its case, lowercase it for matching
	var acts []action.Action
	// ID: what learning is stored under, keep it stable
	// Name, Icon, Group: what's shown
	// Cmd: run with sh -c on pick
	return acts
}
```

Then register its words in `internal/providers/registry.go`:

```go
{[]string{"yourthing", "yt"}, YourThing},
```

To make it reachable from sentences (`set yourthing ...`,
`turn off yourthing`), add a case to `domainProvider` in
`internal/engine/nl.go`.

Things to keep to:

- return `nil` when the tool you depend on isn't installed, so the
  provider just doesn't show up
- quote anything that came from the user or the system with
  `shell.Quote`
- cap the number of results, the function runs on every keystroke
- if a full TUI exists for the area, end with `tuiTail(binary, name, desc)`

If it doesn't need to ship with nZlauncher, a script in `providers.d/`
is less work. See [plugins.md](plugins.md).

## Gotchas

- `nz query` output is tab separated. It strips tabs and newlines from
  the fields, but keep `Cmd` free of them anyway. `clipboard.go` passes
  only the cliphist id and decodes on pick for this reason.
- `hyprctl` can't run Lua binds, they show up as `__lua` with an index.
  `providers/keys.go` reads the `hl.bind(...)` calls from the config
  files and runs them with `hyprctl eval`.
- The GTK face's Wayland app id is the GtkApplication id,
  `dev.tuklu.nz`.
- GTK's SearchEntry eats Escape. The key controller has to use the
  capture phase to see it.
- Don't run a picked command from inside the face. Hand it to
  `shell.LaunchAfterClose` so it runs once the launcher is gone.

## Branches and commits

`dev` is where work happens, `main` is stable. PRs go to `dev`. CI runs
`go vet` and `go build` on every push and PR.

Commits are conventional: `feat:`, `fix:`, `refactor:`, `docs:`,
`chore:`, `test:`. One change per commit.

## Releases

```sh
git checkout main && git merge dev
git tag v0.2.0 && git push origin main v0.2.0
```

Pushing a `v*` tag runs `.github/workflows/release.yml`. It builds on
Ubuntu LTS (older glibc, so the binary runs on more distros) and
attaches `nz-linux-amd64` to the GitHub release. `install.sh` downloads
that file.

## Style

- functional over OOP, stdlib first, tables where they fit
- names that read without comments, comments only for the non-obvious
- new dependencies need a discussion first. Right now it's
  BurntSushi/toml, charmbracelet (TUI) and gotk4 (GTK), nothing else
- no emojis in code or output. Nerd Font glyphs are fine
