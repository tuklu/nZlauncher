# nZlauncher

> The universal entry point for a machine that knows you.

---

## Who this is for

**User**: Jisnu Kalita (tuklu) — backend-first developer, polyglot, power user.  
**Machine**: Xiaomi Notebook Ultra, Arch Linux, Hyprland 0.55.2 (Wayland, native Lua config).  
**Setup**: A custom desktop environment called **neZha** — built by stripping omarchy (a Hyprland layer) down to its metal and owning every script, every keybind, every toggle. No runtime dependencies on upstream frameworks. Everything is a first-class citizen of the machine.

Current stack at time of writing:
- **WM**: Hyprland 0.55.2, native Lua config (`~/.config/hypr/*.lua`)
- **Terminal**: Ghostty (`xdg-terminal-exec` → ghostty)
- **Shell**: zsh
- **Editor**: Neovim (LazyVim)
- **Launcher**: walker (fast but dumb — no context, no intelligence)
- **File manager**: yazi (TUI), nautilus (GUI fallback)
- **Browser**: Chromium — three profiles: Default (Jisnu/personal), Profile 1 (Scalekit Studio/work), Profile 4 (Mahendra)
- **Notifications**: mako
- **Bar**: waybar
- **Key scripts**: `brightness`, `osd`, `lock-screen`, `screenshot`, `launch-screensaver` — all owned, all in `~/.local/bin/` or `~/.local/share/neZha/bin/`
- **Keybinds**: documented at `~/.config/hypr/keybindings.md`

The machine is a known, stable, well-mapped environment. That context is the foundation everything below is built on.

---

## The problem

Every interaction with a computer today routes through one of four things:
- A **GUI app** — mouse-heavy, context-switching, often requires navigating menus
- A **TUI/CLI** — fast but you have to already know the command
- A **launcher** — fast for apps you can name, useless for everything else
- An **LLM chat window** — powerful but slow, stateless, requires context re-injection every time

None of them are sufficient alone. A power user ends up with all four open simultaneously, routing between them based on what kind of task they have. That routing overhead is the real cost.

Concrete examples of the friction:
- Want to open Chromium on your personal profile → you have to remember the `--profile-directory=Default` flag or navigate the profile picker inside the browser
- Want to connect to a saved WiFi network → open impala TUI, navigate menu, select, confirm
- Want to install a package → open terminal, type `yay -S`, remember package name spelling
- Want to find a file you downloaded last week → open yazi, navigate, search
- At 2am stuck on a problem → open a browser tab, type into ChatGPT, re-explain context you've explained a hundred times
- Want to switch audio output → open wiremix TUI, navigate

All of these have known, predictable solutions. The machine already knows how to do all of them. The gap is a fast, intelligent bridge between "what you want to say" and "the command that does it."

---

## The vision

**nZlauncher** is a single entry point that replaces all of the above.

It is not a chatbot. It is not a traditional launcher. It is an **intent-to-action engine** with a fast local brain.

You open it with `SUPER+SPACE`. You type. It figures out what you mean. It does it — or shows you the best match and lets you confirm. Speed of a launcher, flexibility of a shell, awareness of an AI assistant that actually knows your machine.

### What it looks like in practice

```
SUPER+SPACE
[bro_____________________]
  → Chromium · Default (Jisnu)     ← highlighted
    Chromium · Work (Scalekit)
    Chromium · Mahendra
    zen browser
    firefox

SUPER+SPACE
[wifi home______________]
  → Connect · HomeSSID             ← runs nmcli, no navigation
    Connect · PhoneHotspot

SUPER+SPACE
[install that rust formatter_]
  → yay -S rustfmt                 ← inferred from context, shows command
    [Enter to confirm]

SUPER+SPACE
[find my invoice from last week___]
  → ~/Documents/invoices/2026-05-18-scalekit.pdf

SUPER+SPACE
[why is my docker container exiting with 137_____]
  [Shift+Enter] → opens floating terminal with cloud AI, context pre-loaded
```

The first three resolve in under 100ms. The last one invokes a cloud model but the context — machine, user, current task — is already injected.

---

## Architecture

### Three-tier execution stack

```
Input (any text, typos allowed)
        │
        ▼
┌────────────────────────────────────────────────────┐
│  TIER 1 · Static catalog + fuzzy index   <5ms      │
│                                                     │
│  A structured manifest of everything the machine   │
│  can do, indexed at startup, watched for changes.  │
│                                                     │
│  Sources:                                           │
│  · /usr/share/applications + ~/.local/share/apps   │
│  · browser profiles (from Chromium Local State)    │
│  · saved WiFi networks (nmcli / iwctl)             │
│  · shell history (weighted by frecency)            │
│  · recent files (zoxide)                           │
│  · predefined routes (see catalog spec)            │
│                                                     │
│  Matching: fuzzy (fzf-style) + alias table         │
│  If top match score > threshold → surface it       │
└────────────────────┬───────────────────────────────┘
                     │ no confident match
                     ▼
┌────────────────────────────────────────────────────┐
│  TIER 2 · Local small model   50–200ms             │
│                                                     │
│  Ollama · qwen2.5:1.5b or smollm2:1.7b             │
│                                                     │
│  Input:  raw user text + catalog route list        │
│  Output: route ID + extracted arguments            │
│                                                     │
│  System prompt includes machine context JSON:      │
│  user, hostname, distro, apps, profiles, SSIDs,    │
│  common paths, recent activity                     │
│                                                     │
│  The model does NOT decide what to run.            │
│  It maps intent → catalog route. The catalog       │
│  decides what runs. The model just bridges the     │
│  gap between messy human input and clean routes.   │
└────────────────────┬───────────────────────────────┘
                     │ still ambiguous OR
                     │ user presses Shift+Enter
                     ▼
┌────────────────────────────────────────────────────┐
│  TIER 3 · Cloud AI   (Anthropic / configurable)    │
│                                                     │
│  Full reasoning, multi-step, coding, writing.      │
│  Runs in a floating Ghostty terminal, not inline.  │
│  Machine context JSON is pre-injected as prompt.   │
│  This is the "2am" tier.                           │
└────────────────────────────────────────────────────┘
```

### The catalog

The catalog is the core data structure — a TOML manifest that maps everything the machine can do into structured, queryable entries. It is the "known universe" for Tier 1 and the "action space" for Tier 2.

Each entry has:
- `name` — human-readable label shown in results
- `aliases` — the words a person would use to mean this thing (typos welcome, the fuzzy layer handles them)
- `action` — what actually runs (exec, builtin handler, or shell command template)
- `args` — optional: how to extract arguments from user input
- `preview` — optional: what to show in the result pane

```toml
# Browser entries (auto-generated from Chromium Local State)
[[entry]]
name    = "Chromium · Jisnu (personal)"
aliases = ["bro", "browser", "chrome", "personal", "chromium"]
action  = "exec"
cmd     = "chromium --profile-directory=Default --new-window"

[[entry]]
name    = "Chromium · Work (Scalekit Studio)"
aliases = ["work browser", "work chrome", "scalekit", "work"]
action  = "exec"
cmd     = "chromium --profile-directory='Profile 1' --new-window"

# System routes
[[entry]]
name    = "Connect WiFi"
aliases = ["wifi", "internet", "connect", "network", "hotspot"]
action  = "builtin:wifi_connect"
args    = "network_name"

[[entry]]
name    = "Install package"
aliases = ["install", "get", "add", "yay", "pacman", "package"]
action  = "builtin:package_install"
args    = "package_name"

[[entry]]
name    = "System update"
aliases = ["update", "upgrade", "pacman -Syu", "sync packages"]
action  = "exec"
cmd     = "ghostty -e 'yay -Syu'"

# Files
[[entry]]
name    = "Downloads"
aliases = ["downloads", "downloaded files", "dl"]
action  = "exec"
cmd     = "ghostty -e 'yazi ~/Downloads'"
```

The catalog is generated in two ways:
1. **Static** — hand-authored routes for things that are always there (browser profiles, system commands)
2. **Auto-generated** — scanned at startup from desktop files, WiFi networks, mount points, recent directories

### Machine context JSON

This is what gets injected into the Tier 2 model's system prompt. Generated at startup, refreshed periodically.

```json
{
  "user": "tuklu",
  "hostname": "xiaomi-nb-ultra",
  "distro": "arch",
  "shell": "zsh",
  "editor": "nvim",
  "terminal": "ghostty",
  "browser_profiles": [
    { "dir": "Default",   "name": "Jisnu",    "email": "jisnukalita@outlook.com" },
    { "dir": "Profile 1", "name": "Work",     "email": "admin@scalekit.studio"   },
    { "dir": "Profile 4", "name": "Mahendra", "email": "mahendrabarman1@outlook.com" }
  ],
  "wifi_networks": ["HomeSSID", "PhoneHotspot", "OfficeWifi"],
  "common_paths": {
    "screenshots": "~/Pictures/Screenshots",
    "downloads":   "~/Downloads",
    "notes":       "~/Notes",
    "projects":    "~/Development"
  },
  "running_services": ["hypridle", "waybar", "mako", "chromium"],
  "frecent_dirs": ["~/Development/nZlauncher", "~/.config/hypr", "~/Downloads"]
}
```

---

## What this replaces

| Old interaction | nZlauncher interaction |
|---|---|
| Walker → type app name → Enter | Same speed, also handles everything below |
| Chromium → click avatar → switch profile | type `bro` → arrow key → Enter |
| Open impala TUI → navigate → select WiFi | type `wifi home` → Enter |
| Open terminal → `yay -S X` | type `install X` → Enter to confirm |
| Open yazi → navigate to Downloads | type `downloads` or `find X` |
| Open btop | type `activity` or `cpu` |
| Open wiremix → switch audio | type `audio hdmi` or `switch speakers` |
| Open terminal → `hyprctl reload` | type `reload config` |
| Open browser → ChatGPT → re-explain context | `Shift+Enter` → pre-loaded context |

---

## What it is NOT

- Not a replacement for the terminal. Power users still live in the terminal. This is for the cases where you know what you want but the path to it is annoying.
- Not a GUI app manager. It does not install applications with a graphical package browser. It runs `yay -S`.
- Not always-on AI. The model is invoked on-demand, not streaming in the background. Tier 1 (no model) handles the majority of real usage.
- Not a chatbot. There is no conversation history in the launcher itself. That is what Tier 3 (cloud AI in terminal) is for.

---

## Build plan

### Phase 0 — Catalog + fuzzy UI (no AI, immediately useful)
- Static catalog TOML with browser profiles, WiFi, common routes
- Floating UI (walker plugin or custom Rust/Go binary)
- Fuzzy match against catalog aliases
- Browser profile switching works on day one

### Phase 1 — Builtin action handlers
- `wifi_connect` — enumerate saved networks, connect via nmcli
- `package_install` — extract package name, run yay, show output in floating terminal
- `file_search` — fd + fzf, preview in yazi or xdg-open
- `sys_update` — run yay -Syu in a floating terminal

### Phase 2 — Local model tier
- Ollama setup with qwen2.5:1.5b
- Machine context JSON generation script
- Tier 2 fallback: raw text → model → route + args → execute same builtin handlers
- Typo tolerance, natural phrasing, intent extraction

### Phase 3 — Auto-catalog generation
- Parse Chromium Local State for profiles (already know this structure)
- nmcli / iwctl for saved WiFi networks
- Desktop file scanner for installed apps (replacing walker's app index)
- Zoxide integration for frecent directories

### Phase 4 — Cloud tier + context injection
- `Shift+Enter` in launcher → open floating ghostty with aichat/claude
- Machine context JSON pre-injected into system prompt
- Optional: paste current launcher query as initial user message

---

## Name

**nZlauncher** — part of the neZha machine identity. The `nZ` prefix keeps it in the same namespace as the broader neZha project.

Binary name: `nzl` or `nz`. Invoked on `SUPER+SPACE`, replacing walker.

---

## Reference

- Machine config: `~/.config/hypr/`
- Session log: `~/.config/hypr/NEZHA_SESSION_LOG.md`
- Keybindings: `~/.config/hypr/keybindings.md`
- Chromium profile map: Default=Jisnu, Profile 1=Work, Profile 4=Mahendra
- Chromium singleton fix: `autostart.lua` pre-starts `chromium --profile-directory=Default --no-startup-window`
