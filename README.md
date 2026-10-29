# nZlauncher

A launcher for Linux that learns how you use it and takes plain English.
One box for apps, windows, files, answers and system controls, ranked by
what you actually open and when. No AI, no network for any of it (except
currency rates, once a day, if you want them).

Alpha. Wayland, Hyprland first; the rest of the system is detected, so it
works with iwd or NetworkManager, mako or swaync or dunst, and so on.

```
 ╭──────────────────────────────────────────────╮
 │ 󰍉  pdfs from last week bigger than 1mb       │
 ╰──────────────────────────────────────────────╯
  ▌ 󰈦  invoice-march.pdf        ~/Downloads · 6d · 2.1 MiB
    󰈦  tax-2025.pdf             ~/Documents · 3d · 1.4 MiB
```

## What it does

**Learns.** The empty box already shows your likely next pick, from how
often and how recently you open things, the time of day, what you just
launched, the wifi you're on and the app you came from. All local.

**Takes sentences, in any order, with typos.** `wifi off`, `disable
bluetooth`, `louder`, `turn up the volume`, `night light 3500`, `keep
awake for 2 hours`, `performance mode`, `remind me to call mum in 1h`.

**Finds files without syntax.** `screenshots today`, `pdfs from last
week`, `invoice docs bigger than 5mb`, `images in downloads`, `from 4 jan
to 5 march`. An in-memory index of your home answers in well under a
millisecond, ranked like fzf, files you open first. A strong match shows
up in a normal search too, no prefix needed.

**Answers inline.** `5 km to miles`, `44 degrees` (your local unit),
`100 usd to inr`, `time in tokyo`, `3pm london to new york`, `days until
christmas`, `2+2`. Enter copies.

**Controls the machine.** Wifi and bluetooth over D-Bus, volume,
brightness, outputs, workspaces, do not disturb, nightlight, keep awake,
power profiles, clipboard history with image previews, every keybind with
a readable combo, emoji, kill a process.

**Adapts to the setup.** `nzlauncher setup` shows what it found
(compositor, wifi, power, notifications, idle, nightlight) and which
command each action will run. Anything can be overridden in `config.toml`:
a backend, a command (`terminal = "foot -e {cmd}"`), or a whole provider
switched off.

**Two faces, one engine.** A GTK4 window with real icons and a preview
pane, or a terminal UI that shows previews too (images in Ghostty and
kitty). A small daemon keeps everything warm, so a keystroke costs well
under a millisecond. Anything else can drive it over a socket.

## Install

Arch:

```sh
paru -S nzlauncher-bin
systemctl --user enable --now nzlauncher
```

Anywhere else, the prebuilt binaries:

```sh
curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/dev/install.sh | bash
```

Or build it (needs go, and gtk4 + gtk4-layer-shell dev files for the GTK
face):

```sh
git clone https://github.com/tuklu/nZlauncher && cd nZlauncher
go build -o ~/.local/bin/nzlauncher ./cmd/nzlauncher
go build -o ~/.local/bin/nzlauncher-gtk ./cmd/nzlauncher-gtk
```

Then bind it. Lua config:

```lua
hl.bind("SUPER + SPACE", hl.dsp.exec_cmd("nzlauncher bar"), { desc = "nZlauncher" })
```

hyprlang config:

```conf
bind = SUPER, SPACE, exec, nzlauncher bar
```

The first run writes `~/.config/nzlauncher/` with every setting commented
out at its default. Optional tools each switch something on: cliphist and
wl-clipboard (clipboard), iwd or NetworkManager (wifi), hyprsunset
(nightlight), power-profiles-daemon or tlp-pd (power), grim and slurp
(screenshots), ghostty (terminal face).

## How it compares

| | nZlauncher | Vicinae | Walker | rofi, fuzzel |
|---|---|---|---|---|
| Learns your habits | time of day, wifi, the app you came from | launch history | launch history | launch history (drun) |
| Plain sentences | yes, any order, typos | no | no | no |
| File search | own index, sentences | indexed, full text | browser | no |
| System controls | wifi, bt, audio, power, dnd, nightlight | through extensions | a few providers | no |
| Extensions | scripts in any language | React, Raycast store | Elephant providers, menus | scripts, dmenu |
| Written in | Go | C++, Qt | Rust, GTK4 | C |

Vicinae is the one to look at if you want Raycast's extension store.
nZlauncher goes the other way: no extension runtime, everything built in
and offline, and sentences instead of commands to remember.

## Docs

| | |
|---|---|
| [Usage](docs/usage.md) | searching, prefix words, sentences, files, answers, fallbacks |
| [Configuration](docs/configuration.md) | config.toml, backends, commands, catalog, themes |
| [Hyprland](docs/hyprland.md) | binds, window and layer rules |
| [Plugins](docs/plugins.md) | writing your own providers |
| [How it works](docs/how-it-works.md) | routing, ranking, the file index, the learning model, speed |
| [Protocol](docs/protocol.md) | the daemon socket, for writing your own frontend |
| [Development](docs/development.md) | building, code layout, tests, releases |

## License

MIT
