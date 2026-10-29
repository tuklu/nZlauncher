# Usage

## Opening it

`nzlauncher bar` toggles the launcher. If it's already open, running it again
closes it. You can prefill the query, which is handy for dedicated keys:

```sh
nzlauncher bar "power "
nzlauncher bar "clip "
nzlauncher bar "keys "
```

Other commands:

| Command | What it does |
|---|---|
| `nzlauncher bar [query]` | toggle the launcher with the face set in the config |
| `nzlauncher tui [query]` | run the terminal face in the current terminal |
| `nzlauncher gtk [query]` | open the GTK face directly |
| `nzlauncher query [text]` | print results as TSV (`id`, `cmd`, `display`), for scripts |
| `nzlauncher pick [-p prompt] [-i]` | dmenu replacement, see below |
| `nzlauncher dmenu [dmenu flags]` | drop-in for dmenu, rofi `-dmenu`, walker `--dmenu` |

## Keys

| Key | TUI | GTK |
|---|---|---|
| move | up/down, ctrl+p/n, ctrl+k/j | up/down |
| run | enter | enter, or click |
| clear | ctrl+u | |
| close | esc, ctrl+c | esc |

When you pick something, the launcher closes first and the command runs
after the window is gone. That keeps it out of screenshots, OCR and
color pickers, and actions that send keys to the active window hit the
right window.

## Empty query

The first screen shows 8 items: the ones you're most likely to want
right now, based on frequency, recency, time of day, what you just
launched, your network and the app you came from. On a fresh install
it's padded out with defaults.

## Searching

Typing ranks the catalog, installed apps and open windows together.
Every word you type has to match somewhere (name, keywords or group),
so `lib cal` finds LibreOffice Calc. One typo is forgiven from five
letters on, so `chromuim` still finds Chromium.

Apps are also searchable by what they are, not only their name.
Generic names, desktop keywords and categories are all indexed, so
`email` finds Thunderbird and `browser` finds Firefox.

Open windows show up next to the app, so typing `ghostty` gives you
both "switch to it" and "open another one".

Type a group name to list it: `power`, `toggle`, `capture`, `info`, or
`apps` for every application.

## Prefix words

The first word can send the query to one provider:

| Type | You get |
|---|---|
| `wifi`, `net` | known networks to connect or disconnect, on/off toggles |
| `bt`, `bluetooth` | paired devices and their connection state, power toggle |
| `audio`, `output` | output devices, current one marked |
| `vol 40`, `vol +10`, `volume` | set volume directly or by a step, presets, mute |
| `bright 60`, `bright -10`, `brightness` | backlight directly or by a step, presets |
| `ws 3`, `ws move 3` | jump to a workspace, or move the active window there |
| `monitor`, `display` | hyprmoncfg profiles and auto-detect |
| `kill chrom` | your processes sorted by CPU, pick to kill |
| `clip` | clipboard history, pick to copy again |
| `emoji fire` | 1,784 emoji, pick to copy |
| `file notes`, `f` | file search, reads the rest as a sentence (see below), recent files when empty |
| `keys vol` | every Hyprland keybind, pick to run it |
| `wallpaper`, `wp` | images in the `wallpaper_dir` setting, pick to set |
| `transcode` | pick a media file, then a format and resolution |

If a full TUI for that area is installed (impala, bluetui, wiremix,
btop, hyprmoncfg), the last entry opens it.

## Keybinds

`keys` lists every bind from `hyprctl binds`. Each one is labelled with
its `desc` if it has one, otherwise with what it runs.

Lua configs need extra work, because Hyprland reports every Lua bind as
an opaque callback. nZlauncher reads the `hl.bind(...)` calls in
`~/.config/hypr/*.lua` and matches them up by key combo:

- `hl.dsp.exec_cmd("brightness +5%")` shows and runs `brightness +5%`
- `hl.dsp.window.fullscreen(0)` shows `window.fullscreen(0)` and runs
  through `hyprctl eval`
- `function() actions.window_pop() end` shows `actions.window_pop()`
  and runs through `hyprctl eval`, with the file's `local x = require(...)`
  lines in scope

Two kinds can't be resolved, and you get a notification if you pick one:

- binds made in a loop, like `"SUPER + code:" .. i`
- functions that use a local helper defined in the same file

## Sentences

A fixed grammar, no AI involved, so the same words always do the same
thing in well under a millisecond. Word order doesn't matter, filler
words like `the`, `to`, `my` and `please` are dropped, common synonyms
mean the same (`disable` is `off`, `launch` is `open`, `louder` is
`volume up`), and a typo in a word from five letters on is fixed
(`bluetoth`, `yesterdy`).

| Verb | Example |
|---|---|
| `set`, `change`, `make` | `set volume to 40%`, `set wallpaper to beach`, `set brightness 60` |
| volume, brightness | `volume up`, `volume down 10`, `brightness 40 percent`, `louder`, `dimmer`, `mute` |
| `open`, `launch`, `start` | `open thunderbird`, `open github.com` |
| on and off | `wifi off`, `turn bluetooth on`, `disable bluetooth`, `enable nightlight` |
| `connect`, `disconnect` | `connect mx master`, `disconnect home-5g` |
| `switch`, `go`, `focus` | `switch to workspace 3`, `go to firefox` |
| `find` | `find catalog` (file search) |
| `search`, `google` | `search rust traits` |
| `kill`, `stop`, `quit` | `kill chrome` |
| `remind`, `timer` | `remind me in 20 minutes to stretch`, `remind me to call mum in 1h`, `timer 5m`, `set a timer for half an hour` |

### Files

A sentence about files becomes a search of the file index, no syntax
to remember:

- `pdfs from last week`, `screenshots today`, `invoice docs bigger than 5mb`
- `images in downloads`, `go files in projects`, `big videos`, `small pngs from october`
- `report pdf since monday`, `music older than 1 year`, `tax pdf from 2025`, `recent files`

What it understands:

| Kind | Words |
|---|---|
| type | `pdf`, `docs`, `images`/`photos`/`pictures`, `screenshots` (also only in ~/Pictures/Screenshots), `videos`, `music`/`songs`/`audio`, `spreadsheets`, `slides`, `archives`, `ebooks`, `code`, `configs`, `fonts`, or an extension: `pngs`, `.go`, `go files` |
| when | `today`, `yesterday`, `this week`, `last month`, `past 3 days`, `2 weeks ago`, `monday`, `october`, `2026-10-01`, `in 2025`, `since ...`, `before ...`, `older than 1 year`, `newer than 3 days`, `recent` |
| size | `bigger than 5mb`, `over 1gb`, `> 200kb`, `under 1mb`, `big` (100 MB+), `huge` (1 GB+), `small` (under 1 MB) |
| where | `in downloads`, `on desktop`, `in documents`, `in <any folder in home>`, `in ~/some/path` |

Every other word has to appear in the file name. Results come newest
first, with files in hidden folders after visible ones. "Last week"
reaches back to the start of the previous week, so it covers both
readings of it.

A type word on its own only counts in the plural: `screenshot` still
takes a screenshot, `screenshots` lists them. An app whose name starts
with what you typed stays on top, so an app called Docs is still the
first hit for `docs`.

## Answers

Some queries are answered right away, as the first result. Enter copies
the value.

| Kind | Examples |
|---|---|
| units | `5 km to miles`, `72f in c`, `6 ft in cm`, `how many cm in 6 ft`, `1.5 gb to mib`, `2 cups to ml`, `100 kmh to mph` |
| currency | `100 usd to inr`, `$100 in inr`, `50 euros to usd`, `100 usd inr` |
| time | `time in tokyo`, `tokyo time`, `time in japan`, `3pm in london`, `3pm london to new york`, `15:00 utc in ist` |
| dates | `days until christmas`, `days since 2026-01-01`, `in 30 days`, `10 days ago`, `today + 2 weeks`, `what day is 2026-12-25`, `week number`, a unix timestamp |

Units cover length, mass, volume, temperature, data (`gb` is 1000³,
`gib` 1024³), time, speed and area. Currencies are the ~30 the European
Central Bank publishes, by code or by name (`dollars`, `rupees`, `yen`);
the rates are refreshed by the daemon once a day and work offline. Places
are every city with its own time zone, every country, the common
abbreviations (`ist`, `pst`, `cet`) and a few big cities without a zone
(`delhi`, `san francisco`, `beijing`).

## Fallbacks

When the query doesn't match anything well, these are added at the end:

- `5*8+2` gives `= 42`, enter copies it
- `github.com` opens in the browser
- `btop` (anything that's a binary on PATH) runs in a floating terminal
- anything else becomes a web search

## nzlauncher pick

A dmenu replacement for your own scripts. It reads lines from stdin,
shows them in fzf inside a floating ghostty window, and prints the
pick.

```sh
choice=$(printf 'one\ntwo\nthree\n' | nzlauncher pick -p "Number")
name=$(nzlauncher pick -i -p "Name" < /dev/null)    # -i returns typed text if nothing matches
```

The window class is `dev.tuklu.nzlauncher.pick`, so you can give it its own rules.

### dmenu mode

`nzlauncher dmenu` takes dmenu's flags, so scripts written for dmenu, rofi
`-dmenu` or walker `--dmenu` work unchanged. `-p` sets the prompt; look and
layout flags (`-l`, `-fn`, `-nb`, ...) are accepted and ignored. Enter prints
the selection, or what you typed when nothing matches, and cancelling exits 1.
Linked as `dmenu`, the binary behaves this way without the subcommand:

```sh
ln -s /usr/bin/nzlauncher ~/.local/bin/dmenu
```
