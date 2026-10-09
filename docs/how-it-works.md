# How it works

There's no daemon and no cache. Every keystroke runs one function,
`engine.Query`, which reads the catalog, scans `.desktop` files, asks
Hyprland for open windows, replays the event log and ranks everything.
Both faces and `nz query` call that same function.

## Query routing

The query goes to the first route that applies:

```
""                 top 8 items by frecency + context
"apps"             every app, by frecency
a group name       that group (power, toggle, capture, info), by frecency
a sentence         the verb grammar sends it to a provider       + fallbacks
a prefix word      built-in provider, else providers.d plugin    + fallbacks
anything else      rank(catalog + apps + windows)                + fallbacks
```

A sentence needs at least two words after the filler words are dropped,
and the first one has to be a known verb. If the grammar comes back
empty, routing carries on to the prefix and ranking routes.

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

`~/.local/share/neZha/events.jsonl`, one JSON line per pick:

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
