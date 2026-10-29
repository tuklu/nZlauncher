# Protocol

`nzlauncher serve` keeps the engine resident and answers on a unix socket.
Any frontend can drive it: the built-in faces, a Quickshell or AGS widget, a
dmenu-style script, a bar module.

```sh
systemctl --user enable --now nzlauncher
```

## Socket

`$XDG_RUNTIME_DIR/nzlauncher.sock`, mode `0600`, so only your user can talk
to it. One JSON object per line in each direction. A connection can carry
any number of requests.

## Requests

```json
{"op": "query", "q": "set volume 40", "focused": "firefox"}
{"op": "ping"}
```

- `q` is the text in the input box, empty for the first view.
- `focused` is the class of the window the user came from. It feeds the
  context ranking; leave it out if you don't know it.

## Responses

```json
{"ok": true, "results": [
  {"id": "vol:40", "name": "Volume 40%", "icon": "󰕾", "group": "volume",
   "cmd": "wpctl set-volume @DEFAULT_AUDIO_SINK@ 0.40", "display": "󰕾  Volume 40%"}
]}
{"ok": true, "version": "v0.3.0"}
{"ok": false, "error": "catalog: action 3 missing id, name, or cmd"}
```

Results are already ranked. `display` is `icon` and `name` joined the way
the built-in faces show them. `hint` is the short text for a right-hand
column (a key combo, a size, a date; show `group` when it is missing), and
`preview`, when present, is a file worth showing beside the list: an image
or a text file. To run a result, run `cmd` with `sh -c`.

## Learning

Picks are learned from `~/.local/share/nzlauncher/events.jsonl`, one JSON
object per line: `{"id": "...", "ts": 1793137111, "q": "what was typed"}`.
Append a line when the user runs something and the daemon picks it up on
the next query. The built-in faces do this themselves.

## Without the daemon

`nzlauncher query <text>` asks the daemon when it runs and falls back to
running the engine in-process, printing `id<TAB>cmd<TAB>display` lines.

## The GTK face

With `interface = "gtk"`, the first `nzlauncher bar` starts a resident
`nzlauncher-gtk --resident`. It hides instead of quitting, and every later
press only toggles it, so it shows in a few tens of milliseconds. It listens
on `$XDG_RUNTIME_DIR/nzlauncher-gtk.sock` (mode `0600`) for one request:

```json
{"op": "toggle", "q": "prefilled query", "focused": "firefox"}
```

It shows with that query when hidden and hides when shown.
