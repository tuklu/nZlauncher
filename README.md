# nZlauncher

A launcher that learns. One input for everything your computer does —
apps, windows, files, system control, natural language — ranked by what
you actually use, when you use it.

Built for Hyprland on Wayland. Alpha.

```
 ╭──────────────────────────────────────────────╮
 │ 󰍉  remind me in 30 to check the oven         │
 ╰──────────────────────────────────────────────╯
  ▌ 󰔛  Remind in 30m: check the oven        info
```

## Highlights

- **It learns** — frecency, hour-of-day patterns, launch sequences,
  network and focused-app context, and your typed shorthand. All from a
  local event log. Nothing leaves your machine.
- **It never dead-ends** — unmatched queries become intents: inline
  calculator, URL opener, run-in-terminal, web search.
- **It speaks** — `set wallpaper to beach`, `turn off wifi`,
  `connect mx master`, `switch to workspace 3`, `remind me in 20 to stretch`.
- **Two faces, one brain** — a themeable terminal TUI and a native GTK4
  window with real app icons. Pick per config, switch any time.
- **Extensible in any language** — drop an executable in a directory,
  it becomes a provider.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/main/install.sh | bash
```

The installer prefers the prebuilt release (no toolchain needed) and
falls back to building from source. By hand:

```sh
git clone https://github.com/tuklu/nZlauncher
cd nZlauncher
go build -o ~/.local/bin/nz .       # needs go, gtk4 + gtk4-layer-shell dev files
```

Bind it in Hyprland and you're done — first run writes all config:

```conf
bind = SUPER, SPACE, exec, nz bar
```

Runtime requirements: Hyprland, `gtk4` + `gtk4-layer-shell` (GTK face),
a terminal for the TUI face (ghostty assumed). Optional, each enabling
a provider: `cliphist` + `wl-clipboard` (clipboard history),
`bluez-utils` (bluetooth), `iwd` (wifi), `fd` (file search),
`hyprmoncfg` (monitor profiles), `fzf` (the `nz pick` utility).

## Using it

Open the bar and type. The empty view shows your top items — predicted
from frequency, recency, time of day, and what you launched moments ago.
Typing filters everything: applications, open windows, files, actions.

### Providers — type a word, get a domain

| Type | Get |
|---|---|
| `wifi` / `net` | networks with connect/disconnect, on/off toggles |
| `bt` / `bluetooth` | paired devices with live connection state |
| `audio` / `output` | output devices, current one marked |
| `vol 40` / `volume` | direct volume, presets, mute |
| `bright 60` / `brightness` | backlight presets or direct |
| `ws 3` / `ws move 3` | workspace jump / move active window |
| `monitor` / `display` | monitor profiles + auto-detect |
| `kill chrom` | processes by CPU, pick to kill |
| `clip` | clipboard history, pick to re-copy |
| `emoji fire` | 1,784 emoji, pick to copy |
| `file notes` / `f` | file search (recents when bare) |
| `keys vol` | every keybinding, searchable |
| `wallpaper` / `wp` | wallpapers, pick to set |
| `transcode` | media converter: pick file, then format/resolution |
| `apps` | every application |

Several providers end with an "Open <TUI>" escape hatch into a full
manager (impala, bluetui, wiremix, btop) when one is installed.

### Natural language

`set`/`change`, `open`/`launch`, `turn on`/`turn off`,
`connect`/`disconnect`, `switch`/`go`, `find`, `search`, `kill`,
`remind`. Filler words (`the`, `to`, `my`, `me`, …) are ignored:
"turn off the wifi" and "turn wifi off" both work.

### Fallbacks — no dead ends

Anything unmatched becomes: `5*8+2` → inline answer (enter copies),
`github.com` → opens in browser, `btop` → run in a floating terminal
(offered when the binary exists), anything else → web search.

### How ranking works

```
score = name match (40–100)
      + keyword match (up to 50)
      + frecency      (hits × recency weight, cap 50)
      + context bonus (hour affinity, launch sequences,
                       network, focused app — cap 40)
      + learned alias (typed query → past pick, up to 150)
```

The alias layer is the quiet killer: pick Chromium twice after typing
"brw" and "brw" summons Chromium forever — even though it matches
nothing in the name.

## Commands

| Command | What |
|---|---|
| `nz bar [query]` | toggle the launcher, optionally prefilled |
| `nz tui [query]` / `nz gtk [query]` | a face directly |
| `nz query [text]` | engine output as TSV — for scripts |
| `nz pick [-p prompt] [-i]` | dmenu replacement: stdin lines in, pick out |

Prefilled bindings make great dedicated keys:

```conf
bind = SUPER, ESCAPE, exec, nz bar "power "
bind = SUPER CTRL, V, exec, nz bar "clip "
bind = SUPER CTRL, E, exec, nz bar "emoji "
bind = SUPER, K, exec, nz bar "keys "
```

## Configuration

Everything lives in `~/.config/neZha/`, is written with defaults on
first run, and is re-read on every launch — no rebuilds, no reloads.

### `catalog.toml`

```toml
[settings]
interface = "tui"    # tui | gtk
gtk_mode = "overlay" # overlay (above everything) | window (WM-managed)

[[action]]
id = "lock"                    # learning identity — rename `name` freely,
name = "Lock Screen"           #   history follows the id
icon = ""                     # nerd-font glyph
group = "power"                # badge + group listing (type "power")
keywords = ["screen", "afk"]   # extra search terms
cmd = "loginctl lock-session"  # shell command on pick
```

The shipped defaults reference example scripts — point `cmd` at
whatever your system uses.

### Your own providers — `providers.d/`

The filename is the trigger word; it receives the query as `$1` and
prints `id<TAB>command<TAB>display` lines:

```sh
#!/usr/bin/env bash
# ~/.config/neZha/providers.d/pass — type "pass github" in the bar
pass ls | grep -i "$1" | while read -r entry; do
    printf 'pass:%s\tpass -c %q\t󰌾  %s\n' "$entry" "$entry" "$entry"
done
```

Any language. Picks feed the same learning as everything else.

### Themes — `theme/`

`nz-tui.toml` (TUI): colors as ANSI index (follows your terminal
palette), 256-palette, or hex:

```toml
accent = "4"
selection_bg = "4"
prompt_icon = "󰍉"
placeholder_text = "Search anything…"
```

`nz-gtk.css` (GTK): plain GTK4 CSS. Classes: `window.nz`, `.nz-entry`,
`.nz-list`, `.nz-list row`, `.nz-name`, `.nz-badge`.

### Hyprland integration

```conf
# TUI face (floating terminal window)
windowrule = float, class:^nz\.launcher$
windowrule = center, class:^nz\.launcher$
windowrule = size 720 420, class:^nz\.launcher$

# GTK face, window mode
windowrule = float, class:^dev\.tuklu\.nz$
windowrule = center, class:^dev\.tuklu\.nz$
windowrule = size 760 500, class:^dev\.tuklu\.nz$

# GTK face, overlay mode (layer surface)
layerrule = blur, nz-bar
layerrule = ignorealpha 0.1, nz-bar
```

### Data

`~/.local/share/neZha/events.jsonl` is the learning log — timestamp,
typed query, battery, SSID, focused app per pick. Local only. Delete
it to reset the brain.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — repo layout, how the engine
works, adding providers, and the release workflow.

## License

MIT
