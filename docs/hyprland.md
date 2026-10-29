# Hyprland

Examples are in Lua config syntax first, with the hyprlang version
after.

## Binds

`nzlauncher bar` toggles, so one key opens and closes it. A prefilled query
makes a direct shortcut to one provider or group.

```lua
hl.bind("SUPER + SPACE",    hl.dsp.exec_cmd("nzlauncher bar"),             { desc = "nZlauncher" })
hl.bind("SUPER + CTRL + V", hl.dsp.exec_cmd('nzlauncher bar "clip "'),     { desc = "Clipboard manager" })
hl.bind("SUPER + CTRL + E", hl.dsp.exec_cmd('nzlauncher bar "emoji "'),    { desc = "Emoji picker" })
hl.bind("SUPER + CTRL + C", hl.dsp.exec_cmd('nzlauncher bar "capture "'),  { desc = "Capture menu" })
hl.bind("SUPER + ESCAPE",   hl.dsp.exec_cmd('nzlauncher bar "power "'),    { desc = "Power menu" })
hl.bind("SUPER + K",        hl.dsp.exec_cmd('nzlauncher bar "keys "'),     { desc = "Show key bindings" })
```

```conf
bind = SUPER, SPACE, exec, nzlauncher bar
bind = SUPER CTRL, V, exec, nzlauncher bar "clip "
bind = SUPER CTRL, E, exec, nzlauncher bar "emoji "
bind = SUPER CTRL, C, exec, nzlauncher bar "capture "
bind = SUPER, ESCAPE, exec, nzlauncher bar "power "
bind = SUPER, K, exec, nzlauncher bar "keys "
```

The trailing space matters. `"clip "` puts the cursor after the prefix
word so you can start typing a filter right away.

Add a `desc` to your binds. The `keys` provider shows it in place of
the raw command.

## How fast it opens

The GTK face stays resident: the first press after login starts it (well
under a second), every press after that only shows or hides the existing
window. The TUI face starts a fresh ghostty window each time, so it opens as
fast as ghostty starts.

## Window rules

The TUI face is a ghostty window with class `dev.tuklu.nzlauncher.tui`. `nzlauncher pick`
uses `dev.tuklu.nzlauncher.pick`. The GTK face in `window` mode uses `dev.tuklu.nzlauncher`.

```lua
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.tui$" }, float = true })
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.tui$" }, center = true })
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.tui$" }, size = "720 420" })

hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.pick$" }, float = true })
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.pick$" }, center = true })
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.pick$" }, size = "620 360" })

hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher$" }, float = true })
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher$" }, center = true })
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher$" }, size = "760 500" })
```

```conf
windowrule = float, class:^dev\.tuklu\.nzlauncher\.tui$
windowrule = center, class:^dev\.tuklu\.nzlauncher\.tui$
windowrule = size 720 420, class:^dev\.tuklu\.nzlauncher\.tui$

windowrule = float, class:^dev\.tuklu\.nzlauncher\.pick$
windowrule = center, class:^dev\.tuklu\.nzlauncher\.pick$
windowrule = size 620 360, class:^dev\.tuklu\.nzlauncher\.pick$

windowrule = float, class:^dev\.tuklu\.nzlauncher$
windowrule = center, class:^dev\.tuklu\.nzlauncher$
windowrule = size 760 500, class:^dev\.tuklu\.nzlauncher$
```

## Layer rules

The GTK face in `overlay` mode is a layer surface with namespace
`nzlauncher`. Window rules don't reach it.

```lua
hl.layer_rule({ match = { namespace = "^nzlauncher$" }, blur = true })
hl.layer_rule({ match = { namespace = "^nzlauncher$" }, ignore_alpha = 0.1 })
```

```conf
layerrule = blur, nzlauncher
layerrule = ignorealpha 0.1, nzlauncher
```

## Running actions after the launcher closes

When you pick something, the command doesn't start right away. The
launcher hands it to a detached `nzlauncher after-close`, which:

1. waits for the launcher process to exit
2. polls `hyprctl clients` and `hyprctl layers` until no `dev.tuklu.nzlauncher.tui`,
   `dev.tuklu.nzlauncher` or `nzlauncher` surface is left
3. waits another 120ms for the close animation
4. runs the command

It gives up waiting after 2 seconds and runs the command anyway. If
your close animation is long enough that the screenshot tools still
catch the fade-out, turn animations off for the launcher:

```lua
hl.window_rule({ match = { class = "^dev\\.tuklu\\.nzlauncher\\.tui$" }, no_anim = true })
```

```conf
windowrule = noanim, class:^dev\.tuklu\.nzlauncher\.tui$
```

## Two copies on PATH

Hyprland runs binds with its own `PATH`, which can be different from
your shell's. If a bind seems to run an old build, check which
`nzlauncher` it finds:

```sh
tr '\0' '\n' < /proc/$(pgrep -x Hyprland)/environ | grep ^PATH=
which -a nzlauncher
```

Remove the stale one. A leftover `nz` from before the rename can go too;
the binary is `nzlauncher` now, `nz` is only worth keeping as a shell alias
(`alias nz=nzlauncher`), since binds do not see aliases.

## Upgrading from the old names

Before the rename the ids were `nz.launcher`, `nz.pick`, `dev.tuklu.nz` and
the layer namespace `nz-bar`. Rules matching those no longer apply; switch
them to `dev.tuklu.nzlauncher.tui`, `dev.tuklu.nzlauncher.pick`,
`dev.tuklu.nzlauncher` and `nzlauncher`.
