# Plugins

A plugin is any executable in `~/.config/neZha/providers.d/`. The file
name is the prefix word. Type `pass github` and nZlauncher runs
`providers.d/pass github`.

## Contract

- Input: everything after the prefix word comes in as `$1`. It's an
  empty string when you've only typed the word.
- Output: one result per line on stdout, three tab-separated fields:

  ```
  id<TAB>command<TAB>display text
  ```

  `id` is what learning is stored under, so keep it stable for the same
  thing. `command` runs with `sh -c` when picked. `display text` is
  shown as is, so put a glyph at the start if you want an icon.
- Lines with fewer than three fields are skipped.
- A non-zero exit shows no results.
- The file has to be executable (`chmod +x`). Directories and
  non-executable files are ignored.

Built-in prefix words and sentence verbs (`open`, `set`, `find`, ...)
are checked first, so a plugin called `wifi` or `find` never runs.

Picks from a plugin are logged and learned like everything else. The
usual fallbacks (calculator, web search, ...) are added after your
results.

The script runs on every keystroke, so keep it fast. If the data is
slow to get, cache it.

## Examples

pass entries, pick to copy the password:

```sh
#!/usr/bin/env bash
# ~/.config/neZha/providers.d/pass
set -euo pipefail
cd "${PASSWORD_STORE_DIR:-$HOME/.password-store}"
find . -name '*.gpg' | sed 's|^\./||; s|\.gpg$||' | grep -i -- "${1:-}" | head -30 |
while read -r entry; do
    printf 'pass:%s\tpass -c %q\t󰌾  %s\n' "$entry" "$entry" "$entry"
done
```

SSH hosts from your config, pick to open a terminal:

```sh
#!/usr/bin/env bash
# ~/.config/neZha/providers.d/ssh
set -euo pipefail
awk '/^Host / && $2 !~ /[*?]/ { print $2 }' ~/.ssh/config | grep -i -- "${1:-}" |
while read -r host; do
    printf 'ssh:%s\tghostty -e ssh %q\t󰣀  %s\n' "$host" "$host" "$host"
done
```

Not bash only. Same thing in Python:

```python
#!/usr/bin/env python3
# ~/.config/neZha/providers.d/proj
import pathlib, sys

query = sys.argv[1].lower() if len(sys.argv) > 1 else ""
for d in sorted((pathlib.Path.home() / "Development").iterdir()):
    if d.is_dir() and query in d.name.lower():
        print(f"proj:{d.name}\tghostty --working-directory={d}\t  {d.name}")
```

## Test it without the launcher

```sh
~/.config/neZha/providers.d/ssh prod      # raw plugin output
nz query ssh prod                         # what the launcher will show
```

## When to write a built-in instead

If it should ship with nZlauncher, or needs to be part of the sentence
grammar (`turn off ...`, `set ...`), it has to be Go code. See
[development.md](development.md).
