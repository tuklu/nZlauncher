# Tasks

## Launcher polish (2026-10-09)

- [x] Run picked action only after the launcher window is gone (TUI + GTK)
- [x] Keymaps: resolve Lua binds to readable labels and make them runnable
- [x] Apps: index GenericName and Keywords, add Email/Chat/FileManager categories
- [x] Configurable icon library with a built-in Nerd Font map
- [x] README: document icon library and keymap behavior

## Go layout refactor (2026-10-09)

- [x] Move to cmd/nz + internal/{action,engine,providers,learn,config,icons,shell,sysinfo,ui/tui,ui/gtk}
- [x] Update CI, release, install.sh, README, CONTRIBUTING build paths
- [x] Verify `nz query` output identical on 27 queries before/after
