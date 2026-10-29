# Configuration

Everything lives in `~/.config/nzlauncher/`. Missing files are written on first
run, and every file is checked for changes on each query, so edits apply
right away. No rebuild, no reload.

```
~/.config/nzlauncher/
  config.toml           settings, every one commented with its default
  catalog.toml          your static actions
  icons/nerd.toml       built-in icon library (copy it to make your own)
  theme/nz-tui.toml     TUI colors
  theme/nz-gtk.css      GTK styling
  providers.d/          your own providers, see plugins.md
```

## config.toml

Every setting is optional. The first run writes the file with each line
commented out and showing its default, so it doubles as the reference.

```toml
interface = "tui"       # face `nzlauncher bar` opens: tui | gtk
gtk_mode = "overlay"    # GTK face: overlay | window
icon_library = "nerd"   # TUI glyphs: icons/<name>.toml
error_sound = "off"     # Backspace on an empty box: off | system | a sound file

[wallpaper]
dir = "~/Pictures/Wallpapers"
cmd = "swaybg -m fill -i"   # the image path is appended

[search]
url = "https://www.google.com/search?q=%s"

[providers]             # false turns one off everywhere
emoji = false

[fallbacks]             # what an unmatched query turns into
run = false             # calc, url, run, search
```

`error_sound` is off by default. `"system"` lets GTK ring the desktop's
bell, a path plays that file with pw-play, paplay or canberra-gtk-play.

`gtk_mode = "overlay"` is a layer-shell surface above everything, including
fullscreen windows; compositor layer rules apply to it. `"window"` is a
normal window, so window rules and animations apply.

A provider turned off disappears from its prefix word, from sentences that
use it and from the first view. The providers are `apps`, `windows`,
`bluetooth`, `wifi`, `audio`, `volume`, `brightness`, `monitor`,
`workspace`, `wallpaper`, `clipboard`, `emoji`, `files`, `keys`, `kill`,
`transcode`, `plugins` (everything in `providers.d/`), the system tweaks
`power`, `dnd`, `nightlight` and `awake`, and the inline answers `units`,
`currency`, `time` and `dates`.

`currency` is the only part that touches the network: the daemon fetches
the European Central Bank's reference rates from
[Frankfurter](https://frankfurter.dev) (open source, no key) once a day
into `~/.cache/nzlauncher/rates.json`. A query only reads that file, so
conversions work offline with the last rates, and the hint says how old
they are. `currency = false` stops the fetching too.

### Backends

What drives each part of the system is found from the running services,
not guessed from the distro: a Wi-Fi service on the system bus (iwd,
NetworkManager), the compositor from its socket variables, the power
daemon, the notification daemon by the name it reports. `nzlauncher setup`
prints what it found:

```
compositor     hyprland
wifi           iwd
power          none found
notifications  mako
nightlight     hyprsunset
idle           hypridle
               TLP is installed: its tlp-pd package adds power profile switching
distro         arch
```

A power daemon that starts on demand counts as found unless its unit is
masked, and is named by what really serves the API (power-profiles-daemon,
tlp-pd or tuned-ppd). `idle` is whatever serves
`org.freedesktop.ScreenSaver`, which is what keep awake talks to.

When NetworkManager runs it is the one used for Wi-Fi, even when it drives
iwd underneath. `[backends]` overrides any of them:

```toml
[backends]
wifi = "networkmanager"
```

On NixOS and ostree systems (Fedora Silverblue and friends) the setup is
marked immutable, and tweaks stay runtime changes.

The look comes from `theme/nz-gtk.css` and `theme/nz-tui.toml` when they
exist, otherwise the built-in theme. nZlauncher never writes into `theme/`,
so a desktop theme can link the whole directory in.

Before config.toml, settings lived in a `[settings]` block in catalog.toml.
That block is still read, and anything in config.toml takes precedence.

## catalog.toml

### Actions

Each `[[action]]` is one entry you can search for and run:

```toml
[[action]]
id = "lock"
name = "Lock Screen"
icon = ""
group = "power"
keywords = ["screen", "afk"]
cmd = "loginctl lock-session"
```

| Field | Required | |
|---|---|---|
| `id` | yes | what learning is stored under. Keep it stable. You can rename `name` freely and the history stays |
| `name` | yes | what's shown and matched first |
| `cmd` | yes | shell command run with `sh -c` when picked |
| `icon` | no | a Nerd Font glyph |
| `group` | no | shown as the badge on the right. Typing the group name lists the whole group |
| `keywords` | no | extra words to match on |
| `requires` | no | programs the command needs besides its first word, like `["hyprpicker"]` for `pkill hyprpicker \|\| hyprpicker -a` |
| `unless` | no | hide the entry when one of these is installed, for a fallback that gives way to a better tool: `unless = ["screenshot"]` |

An entry whose programs aren't installed is hidden rather than failing
when picked: the first word of `cmd` (past any `VAR=value`) and everything
in `requires` have to be on PATH. That's checked as you type, so a tool
installed later shows up within a minute.

The defaults include entries for helper scripts some setups have
(`screenshot`, `ocr`, `screenrecord`, `battery-status`). Screenshots, OCR
and recording also have a second, plain version on grim and slurp,
tesseract and wf-recorder, marked `unless` the script exists, so either
way there is one entry. Screenshots land in `$XDG_PICTURES_DIR/Screenshots`
and on the clipboard. Point `cmd` at whatever your system uses.

### Commands

What the built-in results run (volume, brightness, terminals, reminders,
opening apps and files) is settled in three steps:

1. **Yours:** the `[commands]` table in config.toml.
2. **Detected:** neZha's scripts when they are there (`osd`, `brightness`,
   `launch-tui`, `reminder`), otherwise the first installed of
   swayosd-client, wpctl, pactl for volume; swayosd-client, brightnessctl
   for brightness; xdg-terminal-exec, `$TERMINAL` or a known terminal;
   a systemd user timer with notify-send for reminders; `uwsm-app` when
   uwsm is installed.
3. **Fallback:** the plain command, or nothing when it can't work (no
   terminal means no terminal results).

```toml
[commands]
terminal = "foot -e {cmd}"
volume_up = "pamixer --increase {n}"
volume_down = "pamixer --decrease {n}"
open = "handlr open {target}"
```

| Key | Placeholders |
|---|---|
| `app` | `{cmd}`: the app's command line |
| `open` | `{target}`: a file or URL, quoted for you |
| `terminal` | `{cmd}`: what to run in it |
| `volume`, `volume_up`, `volume_down` | `{n}`: percent, always positive |
| `mute`, `unmute` | none |
| `brightness`, `brightness_up`, `brightness_down` | `{n}`: percent, always positive |
| `remind` | `{minutes}`, `{text}` (quoted for you) |

`nzlauncher setup` lists every command and what decided it: `config.toml`,
the program it found, or `fallback`.

## Icon libraries

The TUI shows a glyph next to each app. The glyphs come from an icon
library, a TOML file in `~/.config/nzlauncher/icons/`.

The built-in one is `nerd.toml`. It maps about 250 common apps (plus
common web apps like YouTube or Notion) and every freedesktop category
to Nerd Font glyphs, all checked against the Nerd Fonts 3.4 glyph list.

To use your own, create `~/.config/nzlauncher/icons/<name>.toml` and set
`icon_library = "<name>"`. You only need the entries you want to
change. Everything else falls back to the built-in set:

```toml
[apps]
thunderbird = "󰇮"
"org.gnome.Nautilus" = "󰉋"
"yt music" = "󰎆"

[categories]
Game = "󰊗"

[default]
app = "󰣆"
```

For each app, `[apps]` keys are tried in this order:

1. the desktop file id (`org.mozilla.Thunderbird`)
2. the last part of that id after the final dot (`thunderbird`)
3. the `Icon=` value. If it's a file path, the file name without the
   extension is used (`/path/YT Music.png` becomes `yt music`)
4. `StartupWMClass=`
5. the binary name from `Exec=`

Matching ignores case. If nothing in `[apps]` matches, the app's
categories are checked against `[categories]`. More specific
categories are tried first, so an app in `Network;Email;` gets the
Email glyph. If that fails too, `[default] app` is used.

`nerd.toml` in your config dir is a copy of the built-in, written
there as a reference. Editing it works, but it won't get new entries
when nZlauncher updates. A separate file of your own changes is easier
to keep around.

The GTK face doesn't use any of this. It shows icons from your GTK icon
theme.

Glyph search: <https://www.nerdfonts.com/cheat-sheet>

## TUI theme

`theme/nz-tui.toml`. Colors can be an ANSI index (`"0"` to `"15"`,
which follows your terminal palette), a 256-color index (`"212"`) or
hex (`"#89b4fa"`). An empty string means the terminal default.

```toml
accent = "4"             # prompt icon, cursor, selection marker
text = ""                # result text
muted = "8"              # placeholder, footer, badges
border = "8"             # search box border
selection_fg = "0"       # selected row text
selection_bg = "4"       # selected row background
prompt_icon = "󰍉"
placeholder_text = "Search anything…"
```

Leave a key out and its default is used.

## GTK theme

`theme/nz-gtk.css` is plain GTK4 CSS. The classes are:

| Selector | What |
|---|---|
| `window.nz` | the window |
| `.nz-entry` | search box |
| `.nz-list` | result list |
| `.nz-list row`, `.nz-list row:selected` | each row |
| `.nz-name` | result name |
| `.nz-badge` | group badge |

## Data

`~/.local/share/nzlauncher/events.jsonl` is the learning log, one line per
pick. See [How it works](how-it-works.md). Delete it to start
learning from scratch.
