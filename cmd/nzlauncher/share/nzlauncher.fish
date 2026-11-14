# nzlauncher completion for fish. Network names come from the running system
# through `nzlauncher wifi list`; everything else is static.

set -l commands bar tui gtk pick dmenu query serve setup wifi tweak focus ws help man completion

complete -c nzlauncher -f
complete -c nzlauncher -s h -l help -d 'Show help'
complete -c nzlauncher -s V -l version -d 'Show the version'

complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a bar -d 'Open the launcher'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a tui -d 'Open the terminal face'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a gtk -d 'Open the GTK face'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a pick -d 'Choose a line from stdin'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a dmenu -d 'A dmenu-compatible picker'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a query -d 'Print results, tab separated'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a serve -d 'Run the daemon'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a setup -d 'Show what was found here'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a wifi -d 'Connect, join, disconnect, list'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a tweak -d 'Power, dnd, nightlight, keep awake'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a focus -d 'Focus a window'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a ws -d 'Go to a workspace'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a help -d 'Show help for a command'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a man -d 'Print the manual page'
complete -c nzlauncher -n "not __fish_seen_subcommand_from $commands" -a completion -d 'Print a completion script'

# wifi
complete -c nzlauncher -n "__fish_seen_subcommand_from wifi; and not __fish_seen_subcommand_from connect join disconnect list" \
    -a 'connect join disconnect list'
complete -c nzlauncher -n "__fish_seen_subcommand_from wifi; and __fish_seen_subcommand_from connect join" \
    -a '(nzlauncher wifi list 2>/dev/null)' -d 'Network'

# tweak
set -l settings power dnd nightlight awake normal
complete -c nzlauncher -n "__fish_seen_subcommand_from tweak; and not __fish_seen_subcommand_from $settings" -a "$settings"
complete -c nzlauncher -n "__fish_seen_subcommand_from tweak; and __fish_seen_subcommand_from power" -a 'power-saver balanced performance'
complete -c nzlauncher -n "__fish_seen_subcommand_from tweak; and __fish_seen_subcommand_from dnd" -a 'on off'
complete -c nzlauncher -n "__fish_seen_subcommand_from tweak; and __fish_seen_subcommand_from nightlight" -a 'off 3500 4000 4500 5000'
complete -c nzlauncher -n "__fish_seen_subcommand_from tweak; and __fish_seen_subcommand_from awake; and not __fish_seen_subcommand_from screen sleep all" \
    -a 'screen sleep all on off'
complete -c nzlauncher -n "__fish_seen_subcommand_from tweak; and __fish_seen_subcommand_from screen sleep all" \
    -a 'on off 1800 3600 7200'

# the rest
complete -c nzlauncher -n "__fish_seen_subcommand_from ws" -a 'move 1 2 3 4 5 6 7 8 9'
complete -c nzlauncher -n "__fish_seen_subcommand_from pick" -s p -r -d 'Prompt'
complete -c nzlauncher -n "__fish_seen_subcommand_from pick" -s i -d 'Accept typed text'
complete -c nzlauncher -n "__fish_seen_subcommand_from help" -a "$commands"
complete -c nzlauncher -n "__fish_seen_subcommand_from completion" -a 'zsh bash fish'
