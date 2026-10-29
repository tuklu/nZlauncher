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
so `lib cal` finds LibreOffice Calc.

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
| `vol 40`, `volume` | set volume directly, presets, mute |
| `bright 60`, `brightness` | backlight direct or presets |
| `ws 3`, `ws move 3` | jump to a workspace, or move the active window there |
| `monitor`, `display` | hyprmoncfg profiles and auto-detect |
| `kill chrom` | your processes sorted by CPU, pick to kill |
| `clip` | clipboard history, pick to copy again |
| `emoji fire` | 1,784 emoji, pick to copy |
| `file notes`, `f` | file search with fd, recent files when empty |
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

A small fixed grammar, no AI involved. Filler words like `the`, `to`,
`my`, `a` and `please` are dropped, so "turn off the wifi" and "turn
wifi off" do the same thing.

| Verb | Example |
|---|---|
| `set`, `change`, `make` | `set volume 40`, `set wallpaper to beach`, `set brightness 60` |
| `open`, `launch`, `start` | `open thunderbird`, `open github.com` |
| `turn on`, `turn off` | `turn off wifi`, `turn bluetooth on`, `turn on nightlight` |
| `connect`, `disconnect` | `connect mx master`, `disconnect home-5g` |
| `switch`, `go`, `focus` | `switch to workspace 3`, `go to firefox` |
| `find` | `find catalog` (file search) |
| `search`, `google` | `search rust traits` |
| `kill`, `stop` | `kill chrome` |
| `remind`, `remember` | `remind me in 30 to check the oven` |

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
