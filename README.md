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

## Why

- **It learns**: frecency, hour-of-day patterns, launch sequences
  ("after OBS you open Spotify"), network/focused-app context, and your
  typed shorthand ("brw" → Chromium after two picks). All from a local
  append-only event log. Nothing leaves your machine.
- **It never dead-ends**: unmatched queries become intents — calculator,
  URL open, run-in-terminal, web search.
- **It speaks**: `set wallpaper to fg1`, `turn off wifi`,
  `connect mx master`, `kill chromium`, `switch to workspace 3`.
- **Two faces, one brain**: a themeable terminal TUI and a native GTK4
  window (overlay or tiled-WM-managed). Same engine underneath.

## Install

Requirements: Go 1.22+, Hyprland. For the TUI face: a terminal
(ghostty assumed). For the GTK face: `gtk4`, `gtk4-layer-shell`.

```sh
curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/main/install.sh | bash
```

Or by hand:

```sh
git clone https://github.com/tuklu/nZlauncher
cd nZlauncher
go build -o ~/.local/bin/nz .
```

First run writes default config files. Bind it in Hyprland:

```conf
bind = SUPER, SPACE, exec, nz bar
```

Optional integrations (each provider degrades silently when its tool is
missing): `cliphist` + `wl-clipboard` (clipboard history), `bluez-utils`
(bluetooth), `iwd` (wifi), `fd` (file search), `hyprmoncfg` (monitor
profiles), `fzf` (the `nz pick` utility).

## Commands

| Command | What |
|---|---|
| `nz bar [query]` | toggle the launcher, optionally prefilled (`nz bar "clip "`) |
| `nz tui [query]` | the TUI face directly (what `bar` runs in a terminal) |
| `nz gtk [query]` | the GTK face directly |
| `nz query [text]` | raw engine output as TSV (`id<TAB>cmd<TAB>display`) — for scripts |
| `nz pick [-p prompt] [-i]` | dmenu replacement: stdin lines in, selection out; `-i` returns typed text |

## Configuration

Everything lives under `~/.config/neZha/`.

### `catalog.toml` — settings and actions

```toml
[settings]
interface = "tui"    # tui | gtk
gtk_mode = "overlay" # overlay (layer-shell, above everything) | window (WM-managed)

[[action]]
id = "lock"                  # stable id — frecency/learning keys on this
name = "Lock Screen"         # displayed text, matched by the engine
icon = ""                   # nerd-font glyph shown in the list
group = "power"              # badge + group queries (typing "power" lists the group)
keywords = ["screen", "afk"] # extra match terms beyond the name
cmd = "loginctl lock-session"  # shell command run on pick
```

Read fresh on every launch — edit and it just works. The shipped
defaults reference scripts from the author's setup; treat them as
examples and point `cmd` at whatever your system uses.

### Built-in providers (prefix words)

`kill`, `wallpaper`/`wp`, `bluetooth`/`bt`, `audio`/`sound`/`output`,
`wifi`/`net`, `volume`/`vol`, `workspace`/`ws`, `brightness`/`bright`,
`file`/`f`, `monitor`/`display`, `clipboard`/`clip`, `emoji`,
`keys`/`binds`, `transcode`/`convert`.

Magic queries: `apps` lists all applications; a group name lists that
group. Natural language verbs: `set/change`, `open/launch`,
`turn on/off`, `connect/disconnect`, `switch/go`, `search`, `find`,
`kill`, `remind`.

### Your own providers — `providers.d/`

Drop an executable in `~/.config/neZha/providers.d/`. The filename is
the trigger word; it receives the rest of the query as `$1` and prints
one action per line:

```
id<TAB>shell command<TAB>display text
```

```sh
#!/usr/bin/env bash
# ~/.config/neZha/providers.d/pass — type "pass github" in the bar
pass ls | grep -i "$1" | while read -r entry; do
    printf 'pass:%s\tpass -c %q\t󰌾  %s\n' "$entry" "$entry" "$entry"
done
```

Any language. Picks feed the same learning as everything else.

### Themes — `theme/`

**TUI** (`theme/nz-tui.toml`): colors as ANSI index (`"4"`, follows your
terminal palette), 256-palette (`"212"`), or hex (`"#89b4fa"`).

```toml
accent = "4"            # prompt, cursor, selection marker
muted = "8"             # placeholder, footer, badges
border = "8"            # search box border
selection_fg = "0"
selection_bg = "4"
prompt_icon = "󰍉"
placeholder_text = "Search anything…"
```

**GTK** (`theme/nz-gtk.css`): plain GTK4 CSS. Classes: `window.nz`,
`.nz-entry`, `.nz-list`, `.nz-list row`, `.nz-name`, `.nz-badge`.

Both files are written with defaults on first run and re-read on every
launch — no rebuild to retheme.

### Data — `~/.local/share/neZha/`

- `events.jsonl` — append-only launch log: timestamp, typed query,
  battery state, wifi SSID, focused app. This is the training data;
  delete it to reset all learning.
- `frecency.json` — legacy aggregate baseline, read-only.

### Hyprland integration

```conf
# the TUI face runs in a floating terminal
windowrule = float, class:^nz\.launcher$
windowrule = center, class:^nz\.launcher$
windowrule = size 720 420, class:^nz\.launcher$

# GTK face in window mode
windowrule = float, class:^dev\.tuklu\.nz$
windowrule = center, class:^dev\.tuklu\.nz$
windowrule = size 760 500, class:^dev\.tuklu\.nz$

# GTK face in overlay mode (layer surface)
layerrule = blur, nz-bar
layerrule = ignorealpha 0.1, nz-bar
```

Prefilled bindings compose well:

```conf
bind = SUPER, ESCAPE, exec, nz bar "power "
bind = SUPER CTRL, V, exec, nz bar "clip "
bind = SUPER CTRL, E, exec, nz bar "emoji "
bind = SUPER, K, exec, nz bar "keys "
```

## License

MIT
