package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
)

// version is set by the release build: -ldflags "-X main.version=0.4.7".
var version = ""

// share holds what the package writes out at install time: the manual page
// and the completion scripts, so a release needs no files besides the binary.
//
//go:embed all:share
var share embed.FS

func versionString() string {
	if version != "" {
		return version
	}
	// go install records the module version; a plain go build says (devel)
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

const mainHelp = `usage: nzlauncher <command> [arguments]
       nzlauncher -h | --help | -V | --version

A launcher that reads what you type as a sentence and learns how you use it.

Faces:
  bar [query]          open the launcher, in the face config.toml picks
  tui [query]          open the terminal face
  gtk [query]          open the GTK face (needs nzlauncher-gtk)

Pickers, for scripts:
  pick [-p prompt] [-i]
                       choose a line from stdin and print it
  dmenu [flags]        a dmenu-compatible picker

Engine:
  query [text]         print results as id, command, display, tab separated
  serve                run the daemon (nzlauncher.service does this)
  setup                show what was found on this machine and what drives it

System:
  wifi <action>        connect, join or disconnect a network; list those in range
  tweak <setting>      power profile, do not disturb, nightlight, keep awake
  catalog <action>     encrypt or decrypt your catalog, edit it either way
  focus <window>       focus a window by class or title
  ws [move] <n>        go to workspace n, or move the focused window there

Help:
  help [command]       this text, or a command's
  man                  print the manual page as roff
  completion <shell>   print a completion script for zsh, bash or fish

Run 'nzlauncher help <command>' for a command's arguments. See nzlauncher(1).
`

// commandHelp is each command's own help. The first line is its synopsis,
// which a usage error repeats.
var commandHelp = map[string]string{
	"bar": `usage: nzlauncher bar [query]

Open the launcher with query typed in, in the face config.toml's interface
setting picks: tui or gtk. Bind this to a key.
`,
	"tui": `usage: nzlauncher tui [query]

Open the terminal face in the current terminal, with query typed in.
`,
	"gtk": `usage: nzlauncher gtk [query]

Open the GTK face, with query typed in. Needs nzlauncher-gtk next to this
binary or on PATH.
`,
	"pick": `usage: nzlauncher pick [-p prompt] [-i]

Read lines from stdin, let the user choose one in a floating picker, and
print it. Prints nothing and exits 0 when the picker is closed.

  -p prompt   the prompt, "Pick" by default
  -i          accept typed text that matches no line

example: ls ~/Pictures | nzlauncher pick -p Wallpaper
`,
	"dmenu": `usage: nzlauncher dmenu [-p prompt] [dmenu flags]

A dmenu replacement for scripts that call dmenu: reads lines from stdin and
prints the choice, or the typed text. Other dmenu flags are accepted and
ignored. Exits 1 when the picker is closed. Linked or copied as "dmenu",
the binary is this command.
`,
	"query": `usage: nzlauncher query [text]

Print the results for text, one per line: id, command and display, separated
by tabs. Asks the daemon, or runs the engine itself when none is running.
An empty text gives the first view.

example: nzlauncher query volume 40 | cut -f2 | sh
`,
	"serve": `usage: nzlauncher serve

Run the daemon: keeps the engine warm, indexes files, holds keep awake.
Listens on $XDG_RUNTIME_DIR/nzlauncher.sock. The systemd user unit
nzlauncher.service runs this.
`,
	"setup": `usage: nzlauncher setup

Show what nzlauncher found on this machine, and what drives each
capability, overrides from config.toml included.
`,
	"wifi": `usage: nzlauncher wifi connect <ssid>
       nzlauncher wifi join <ssid>
       nzlauncher wifi disconnect
       nzlauncher wifi list

Drive Wi-Fi through iwd or NetworkManager.

  connect      a known or open network
  join         a new secured network: asks for the passphrase in a dialog
               (iwd, with zenity or kdialog). Exits 3 when it cannot ask
               here, so the caller can fall back to a terminal, and 4 when
               the dialog was closed.
  disconnect   the current network
  list         the networks in range, one name per line
`,
	"tweak": `usage: nzlauncher tweak power <profile>
       nzlauncher tweak dnd on|off
       nzlauncher tweak nightlight <kelvin>|off
       nzlauncher tweak awake [screen|sleep|all] on|off|<seconds>
       nzlauncher tweak normal

Change a system setting.

  power        a power profile: power-saver, balanced, performance
               (power-profiles-daemon, tlp-pd or tuned-ppd)
  dnd          do not disturb (mako, swaync, dunst)
  nightlight   warm the screen, 1000 to 10000 kelvin (hyprsunset)
  awake        keep the screen on (screen: no lock, no blanking), keep the
               machine from sleeping, lid included (sleep: the screen may
               still lock), or both (all). on, or a number of seconds, holds
               it; off lets go. Without a hold name: screen. Needs the daemon.
  normal       let go of every keep awake hold, balanced power profile

example: nzlauncher tweak awake sleep 7200
`,
	"catalog": `usage: nzlauncher catalog status
       nzlauncher catalog encrypt
       nzlauncher catalog decrypt
       nzlauncher catalog edit

Keep the catalog in plain text (the default) or encrypted, so only
nzlauncher opens it.

  status    plain or encrypted, and where the key comes from
  encrypt   catalog.toml becomes catalog.toml.enc, and the plain file goes
  decrypt   back to catalog.toml
  edit      open it in $VISUAL or $EDITOR; an encrypted one is decrypted to
            $XDG_RUNTIME_DIR (memory) and encrypted again on save

The key comes from [catalog] key in config.toml:
  file          a key nzlauncher makes, in ~/.local/share/nzlauncher/catalog.key
                (the default; keep a copy, nothing opens the catalog without it)
  file:<path>   the same, somewhere of your choosing
  keyring       gnome-keyring, KWallet or KeePassXC
  op://...      a secret in 1Password; reading it asks 1Password
  passphrase    typed once per daemon start

file and keyring keep the catalog safe wherever it is copied or synced, not
from scripts running as you, which can read the key too. op and passphrase
do: the key needs your approval or your memory.
`,
	"focus": `usage: nzlauncher focus <window>

Focus the first window whose class or title matches window.
`,
	"ws": `usage: nzlauncher ws [move] <n>

Go to workspace n, or with move, send the focused window there.
`,
	"help": `usage: nzlauncher help [command]

Show the overview, or a command's arguments.
`,
	"man": `usage: nzlauncher man

Print the manual page as roff. Packages install it; to read it from a
build without one: nzlauncher man | man -l -
`,
	"completion": `usage: nzlauncher completion zsh|bash|fish

Print a completion script. Packages install them; to load one by hand:

  zsh    nzlauncher completion zsh > ~/.local/share/zsh/site-functions/_nzlauncher
  bash   source <(nzlauncher completion bash)
  fish   nzlauncher completion fish > ~/.config/fish/completions/nzlauncher.fish
`,
}

// usageError is a command used wrongly: main prints its synopsis and exits 2.
type usageError struct {
	command string
	reason  string
}

func (e usageError) Error() string { return e.reason }

func misuse(command, format string, args ...any) error {
	return usageError{command, fmt.Sprintf(format, args...)}
}

// wantsHelp reports a -h or --help before any "--".
func wantsHelp(args []string) bool {
	for _, a := range args {
		switch a {
		case "--":
			return false
		case "-h", "--help":
			return true
		}
	}
	return false
}

// withoutDashDash drops the first "--", which only ends the options.
func withoutDashDash(args []string) []string {
	for i, a := range args {
		if a == "--" {
			return append(args[:i:i], args[i+1:]...)
		}
	}
	return args
}

func printHelp(w io.Writer, command string) error {
	if command == "" {
		fmt.Fprint(w, mainHelp)
		return nil
	}
	text, ok := commandHelp[command]
	if !ok {
		return misuse("help", "no command %q", command)
	}
	fmt.Fprint(w, text)
	return nil
}

func runHelp(args []string) error {
	switch len(args) {
	case 0:
		return printHelp(os.Stdout, "")
	case 1:
		return printHelp(os.Stdout, args[0])
	}
	return misuse("help", "help takes one command")
}

// reportUsage prints a usage error the way Unix tools do: what went wrong,
// the synopsis, and where to read more, on stderr.
func reportUsage(err error) bool {
	var u usageError
	if !errors.As(err, &u) {
		return false
	}
	fmt.Fprintf(os.Stderr, "nzlauncher: %s\n", u.reason)
	if text, ok := commandHelp[u.command]; ok {
		fmt.Fprintln(os.Stderr, strings.SplitN(text, "\n\n", 2)[0])
		fmt.Fprintf(os.Stderr, "Try 'nzlauncher help %s'.\n", u.command)
	} else {
		fmt.Fprintln(os.Stderr, "Try 'nzlauncher --help'.")
	}
	return true
}

func runMan(args []string) error {
	if len(args) != 0 {
		return misuse("man", "man takes no arguments")
	}
	page, err := share.ReadFile("share/nzlauncher.1")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(page)
	return err
}

var completionFiles = map[string]string{
	"zsh":  "share/_nzlauncher",
	"bash": "share/nzlauncher.bash",
	"fish": "share/nzlauncher.fish",
}

func runCompletion(args []string) error {
	if len(args) != 1 {
		return misuse("completion", "completion takes one shell")
	}
	file, ok := completionFiles[args[0]]
	if !ok {
		return misuse("completion", "no completion for %q: zsh, bash or fish", args[0])
	}
	script, err := share.ReadFile(file)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(script)
	return err
}
