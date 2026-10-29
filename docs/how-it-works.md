# How it works

There's no daemon and no cache. Every keystroke runs one function,
`engine.Query`, which reads the catalog, scans `.desktop` files, asks
Hyprland for open windows, replays the event log and ranks everything.
Both faces and `nzlauncher query` call that same function.

## Query routing

The query goes to the first route that applies:

```
""                 top 8 items by frecency + context
"apps"             every app, by frecency
a group name       that group (power, toggle, capture, info), by frecency
an answer         put on top of whatever the routes below return
a file sentence    apps named like the query, then the file index  + fallbacks
a sentence         the verb grammar sends it to a provider       + fallbacks
a prefix word      built-in provider, else providers.d plugin    + fallbacks
anything else      rank(catalog + apps + windows)                + fallbacks
```

Sentences go through `internal/intent` first. It splits the query into
words, maps synonyms to one word (`disable` to `off`, `louder` to `volume
up`), fixes one typo in a known word of five letters or more, and pulls
out values: numbers, percentages, durations, sizes, dates and periods,
file types and folders. All of it is plain string work, no model, a few
microseconds.

A file sentence is one with a plural type (`pdfs`), or two signals out of
type, the word "file", a date, a size and a folder, or a folder after "in"
with a name to look for. It becomes an index filter. A verb sentence
starts with a known verb, or names a level (`volume 40%`), or opens or
ends with on/off. If the grammar comes back empty, routing carries on to
the prefix and ranking routes.

## File index

The daemon keeps every file under home in memory, apart from caches,
build output and dependency trees: one entry per file (folder, name,
size, mtime) with folder paths shared, all lowercased names packed into
one buffer, and a list of entries for every trigram and every extension.
A search walks the smallest set that holds every match: the shortest
trigram list of the terms, the files of the asked types, or the run of
entries under the asked folders (the walk is depth first, so a folder's
subtree is one run, found by binary search in the sorted folder list).
Lists over 20,000 entries are split across cores, and only the best 25
are kept while scanning.

The index is rebuilt every 20 minutes and right after a change in home,
Downloads, Desktop, Documents, Pictures, Screenshots, Videos or Music.

## Ranking

Each candidate gets one score and the list is sorted by it. Ties go by
name.

```
score = match + alias + min(frecency, 50) + min(context, 40)
```

An item needs a match or an alias to show up at all.

### Match

The query is split into words and every word has to match, otherwise
the item is dropped. For each word the best of these counts:

| Word matches | Points |
|---|---|
| start of the name | 100 |
| start of any word in the name | 85 |
| anywhere in the name | 70 |
| a keyword | 50 |
| name, letters in order (`ffx` in `firefox`) | 40 |
| keyword, letters in order | 25 |
| one typo in the start of a word, from 5 letters (`chromuim`) | 30 |
| start of the group name | 20 |

The points are added up over all the words.

### Frecency

```
frecency = hits x weight

last used    weight
< 1 hour     4
< 1 day      2
< 1 week     1
older        0.25
```

### Context

Learned from the event log. Every part needs a minimum amount of data
before it does anything, so with only a few launches it adds nothing.

| Signal | Max | Needs |
|---|---|---|
| hour of day: how much of this item's use falls within 2 hours of now, compared to an even spread | 20 | 5 launches of the item |
| sequence: how often you open this right after the last thing you opened | 30 | last pick under 15 min ago, this pair seen at least twice |
| network: share of launches on the current wifi | 10 | 3 launches of the item |
| focused app: share of launches from the app you were in | 15 | 3 launches of the item |

The whole context bonus is capped at 40.

### Aliases

Every pick stores the text you had typed. Once the same text has led to
the same item twice, it becomes an alias:

- typing exactly that text again gives up to 150 points
- typing the start of it (2+ characters) gives up to 70

The points are split by how often each item was picked for that text.

An alias works even when the item doesn't match the text. Pick
Chromium twice after typing `brw` and `brw` brings up Chromium from
then on.

## The event log

`~/.local/share/nzlauncher/events.jsonl`, one JSON line per pick:

```json
{"id":"app:org.mozilla.Thunderbird.desktop","ts":1791547200,"q":"email","ssid":"home-5g","focus":"com.mitchellh.ghostty"}
```

| Field | |
|---|---|
| `id` | the action id |
| `ts` | unix time |
| `q` | what you typed |
| `bat` | on battery (only written when true) |
| `ssid` | current wifi network |
| `focus` | class of the window that had focus before the launcher opened |

All learning is rebuilt from this file on every query. Edit it, trim
it or delete it and the launcher follows. `frecency.json` next to it is
an older format that's still read as a starting point if it exists.

## Action ids

Learning is stored per id, so ids are kept stable:

| Source | id |
|---|---|
| catalog | the `id` you set |
| apps | `app:<desktop file name>` |
| windows | `win:<class>`, so switching to an app is learned across windows |
| providers | `<provider>:<thing>`, like `wifi:home-5g` or `bt:<mac>` |
| plugins | whatever the plugin prints |

## Speed

Measured on an i5-11300H (Hyprland 0.56, 158 desktop entries, 1178 logged
launches), before and after the engine rework in 0.3:

| | 0.2 | 0.3 |
|---|---|---|
| keystroke, empty query | 21.2ms | 0.75ms |
| keystroke, `fire` | 21.2ms | 0.20ms |
| keystroke, `set volume 40` | 11.7ms | 0.07ms |
| `nzlauncher query fire`, one shot | 90ms | 5.7ms (daemon), 21ms (no daemon) |
| CLI startup | 62ms | 4ms |
| bar to visible, GTK face | 1057ms | 22-39ms (resident, after the first press) |
| bar to visible, TUI face | 1489ms | 699ms (mostly ghostty starting) |

Where it comes from: catalog, apps and the learning model stay loaded and
reload only when their files change; the window list comes from Hyprland's
socket and is refetched only after window events; the CLI has no GTK linked
in; the GTK face stays resident and only shows and hides.

File queries against a home of 520,000 files, through the daemon:

| query | time |
|---|---|
| `f catalog` | 0.2ms |
| `pdfs from last week` | 0.13ms |
| `screenshots today` | 0.15ms |
| `pdfs bigger than 5mb` | 0.18ms |
| `recent files` | 1.4ms |
| `go files in development` (126,000 matches) | 2.8ms |

Reproduce the per-stage numbers with:

```sh
go test ./internal/engine -run '^$' -bench . -benchtime 50x
go test ./internal/files -run '^$' -bench Home -benchtime 30x
```
