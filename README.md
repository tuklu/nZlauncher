# nZlauncher

A launcher for Hyprland that learns how you use it. One input box for
apps, windows, files, system controls and plain-English commands, ranked
by what you actually open and when you open it.

Alpha. Wayland and Hyprland only.

```
 ╭──────────────────────────────────────────────╮
 │ 󰍉  remind me in 30 to check the oven         │
 ╰──────────────────────────────────────────────╯
  ▌ 󰔛  Remind in 30m: check the oven        info
```

## What it does

Open it and the list is already your most likely next pick, worked out
from how often and how recently you used things, the time of day, what
you launched a minute ago, the wifi you're on and the app you came from.
All of that comes from a local log file. Nothing goes over the network.

Type and it filters everything at once. A word like `wifi`, `bt`, `vol`,
`clip` or `keys` switches to that domain. Short sentences work too:
`turn off wifi`, `set volume 40`, `open thunderbird`, `connect mx master`.

If nothing matches you still get something useful: a calculator answer,
a URL to open, the command run in a terminal, or a web search.

There are two frontends on the same engine: a terminal UI (themeable,
Nerd Font icons) and a native GTK4 window with real app icons. You pick
one in the config.

Anything not built in can be added as a script in any language. Drop an
executable in `providers.d/` and its name becomes a new prefix word.

## Install

On Arch, build a package from `packaging/`: `arch-bin` installs the release
binary in seconds, `arch` builds from source (the first build compiles the
GTK bindings and takes a while).

```sh
git clone https://github.com/tuklu/nZlauncher && cd nZlauncher/packaging/arch-bin && makepkg -si
```

Anywhere else:

```sh
curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/main/install.sh | bash
```

This grabs the prebuilt release if there is one and builds from source
if not. To build it yourself (needs go, gtk4 and gtk4-layer-shell dev
files):

```sh
git clone https://github.com/tuklu/nZlauncher
cd nZlauncher
go build -o ~/.local/bin/nz ./cmd/nzlauncher
```

or

```sh
go install github.com/tuklu/nZlauncher/cmd/nzlauncher@latest
```

Then bind it. In a Lua config:

```lua
hl.bind("SUPER + SPACE", hl.dsp.exec_cmd("nzlauncher bar"), { desc = "nZlauncher" })
```

In a hyprlang config:

```conf
bind = SUPER, SPACE, exec, nzlauncher bar
```

The first run writes every config file with defaults to
`~/.config/nzlauncher/`.

Runtime needs: Hyprland, gtk4 + gtk4-layer-shell for the GTK face, and
ghostty for the TUI face. Each optional tool turns on one provider:
cliphist + wl-clipboard (clipboard), bluez-utils (bluetooth), iwd
(wifi), fd (file search), hyprmoncfg (monitors), fzf (`nzlauncher pick`).

## Docs

| | |
|---|---|
| [Usage](docs/usage.md) | searching, prefix words, sentences, fallbacks, keybinds |
| [Configuration](docs/configuration.md) | catalog, settings, icon libraries, themes |
| [Hyprland](docs/hyprland.md) | binds, window and layer rules |
| [Plugins](docs/plugins.md) | writing your own providers |
| [How it works](docs/how-it-works.md) | ranking, the learning model, the event log |
| [Protocol](docs/protocol.md) | the engine daemon and its socket, for writing your own frontend |
| [Development](docs/development.md) | building, code layout, adding a provider, releases |

## License

MIT
