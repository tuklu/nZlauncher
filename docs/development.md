# Development

## Build

```sh
# Arch
sudo pacman -S go gtk4 gtk4-layer-shell

# Debian/Ubuntu. gtk4-layer-shell isn't packaged on 24.04,
# .github/workflows/ci.yml has the build-from-source steps.
sudo apt install golang libgtk-4-dev libgirepository1.0-dev

CGO_ENABLED=0 go build -o nzlauncher ./cmd/nzlauncher   # CLI, engine, TUI, daemon: no GTK
go build -o nzlauncher-gtk ./cmd/nzlauncher-gtk          # the GTK face
go vet ./...
```

The first build compiles the gotk4 bindings and takes a few minutes.
After that the build cache makes it quick.

The cgo `conflicting types for built-in function 'free'` warnings come
from gotk4 and are harmless.

To try a change without touching your installed copy:

```sh
go build -o /tmp/nz ./cmd/nzlauncher
/tmp/nz query "turn off wifi"
/tmp/nz tui
```

`nzlauncher query` prints `id<TAB>cmd<TAB>display` for each result, which is
the easiest way to check the engine. Saving its output for a set of
queries before and after a change catches most regressions.

## Tests

```sh
go test ./...                                   # unit tests, no services needed
python3 test/dbusmock/run.py -v ./internal/wifi ./internal/tweaks ./internal/bluez
```

The second runs the D-Bus backends against fake services: it starts
private system and session buses with python-dbusmock's NetworkManager,
power-profiles and BlueZ in a known scene (`pip install python-dbusmock`,
plus dbus-python), then the tests built with `-tags dbusmock`. The real
buses are never touched. Opt-in live tests: `NZ_WIFI_LIVE=1` and
`NZ_BT_LIVE=1` (that one toggles the real adapter).

Benchmarks against the real machine:

```sh
go test ./internal/engine -run '^$' -bench . -benchtime 50x
go test ./internal/files -run '^$' -bench Home -benchtime 30x
```

## Layout

```
cmd/nzlauncher/         CLI: bar, tui, query, serve, setup, wifi, tweak, pick, dmenu
cmd/nzlauncher-gtk/     the GTK face, its own binary so the CLI links no GTK
internal/
  action/               the Action type
  server/               the daemon: socket, protocol, file index and rates upkeep
  engine/               Query: routing, match scoring, ranking, verb grammar, fallbacks
  answer/               units, currency, time zones, date math
  intent/               words, synonyms, typos, slots (sizes, dates), file sentences
  providers/            apps, windows, keys, wifi, bluetooth, audio, files, clipboard,
                        tweaks, emoji, monitor, transcode, plugins, the prefix table
  files/                the in-memory file index and its search
  wifi/ bluez/ tweaks/  services over D-Bus: iwd, NetworkManager, BlueZ, power
                        profiles, notification daemons, hyprsunset, keep awake
  platform/             what runs here: compositor, wifi, power, notifications...
  portable/             the command line each action runs, [commands] first
  learn/                event log, frecency, context model
  config/               config.toml, catalog.toml, defaults
  hypr/                 Hyprland sockets and events
  icons/ identity/      TUI icon libraries; app id and window classes
  shell/ sysinfo/       quoting and detached launch; battery and SSID
  ui/tui/ ui/gtk/       the two faces
test/dbusmock/          fake services for the D-Bus backends
```

`providers` never imports `engine`, so the sentence grammar in `engine`
can call providers by name without an import cycle; `intent` and `answer`
are pure functions with no providers under them.

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
is less work. See [Plugins](plugins.md).

## Gotchas

- `nzlauncher query` output is tab separated. It strips tabs and newlines from
  the fields, but keep `Cmd` free of them anyway. `clipboard.go` passes
  only the cliphist id and decodes on pick for this reason.
- `hyprctl` can't run Lua binds, they show up as `__lua` with an index.
  `providers/keys.go` reads the `hl.bind(...)` calls from the config
  files and runs them with `hyprctl eval`.
- The GTK face's Wayland app id is the GtkApplication id,
  `dev.tuklu.nzlauncher`.
- GTK's SearchEntry eats Escape. The key controller has to use the
  capture phase to see it.
- Don't run a picked command from inside the face. Hand it to
  `shell.LaunchAfterClose` so it runs once the launcher is gone.

## Docs and the wiki

`docs/` is the source. The [wiki](https://github.com/tuklu/nZlauncher/wiki)
is generated from it by `scripts/docs-to-wiki.sh` whenever `docs/` or
`README.md` changes on `main`, so don't edit the wiki directly. To see
the output locally:

```sh
scripts/docs-to-wiki.sh . /tmp/wiki
```

## Branches and commits

`dev` is the default branch and where work happens; releases are tags on
it. PRs go to `dev`. CI runs `go vet`, `go build` and `go test` on every
push and PR, and the dbusmock tests in a second job. The wiki is rebuilt
from `docs/` on every docs change to `dev`.

Commits are conventional: `feat:`, `fix:`, `refactor:`, `docs:`,
`chore:`, `test:`. One change per commit.

## Releases

```sh
git tag v0.4.3 && git push origin v0.4.3
```

Pushing a `v*` tag runs `.github/workflows/release.yml`. It builds on
Ubuntu LTS (older glibc, so the binary runs on more distros) and attaches
`nzlauncher-linux-amd64`, `nzlauncher-gtk-linux-amd64`, a PKGBUILD with
real checksums and `SHA256SUMS` to the GitHub release; `install.sh`
downloads the binaries. Then an Arch container writes `.SRCINFO` and
pushes that PKGBUILD to the AUR as `nzlauncher-bin`, with the key in the
`AUR_SSH_PRIVATE_KEY` secret and AUR's host keys pinned in the
`AUR_KNOWN_HOSTS` variable.

## Style

- functional over OOP, stdlib first, tables where they fit
- names that read without comments, comments only for the non-obvious
- new dependencies need a discussion first. Right now it's
  BurntSushi/toml, charmbracelet (TUI), gotk4 (GTK), godbus (D-Bus) and
  golang.org/x/sys (terminal size), nothing else
- no emojis in code or output. Nerd Font glyphs are fine
