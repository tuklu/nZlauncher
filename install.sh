#!/usr/bin/env bash
set -euo pipefail

# nZlauncher installer
#   curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/main/install.sh | sh
#
# Overrides:
#   NZ_REPO    git url or local path to build from (default: github.com/tuklu/nZlauncher)
#   NZ_PREFIX  install dir (default: /usr/local/bin, falls back to ~/.local/bin)

REPO="${NZ_REPO:-https://github.com/tuklu/nZlauncher}"

say()  { printf '\033[1m[nz]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[nz]\033[0m %s\n' "$*" >&2; exit 1; }

# ── requirements ──────────────────────────────────────────────────────────────
command -v go >/dev/null  || fail "go is required (pacman -S go | apt install golang)"
command -v git >/dev/null || fail "git is required"
command -v pkg-config >/dev/null || fail "pkg-config is required"

pkg-config --exists gtk4 2>/dev/null ||
    fail "gtk4 dev files required (pacman -S gtk4 | apt install libgtk-4-dev)"
pkg-config --exists gtk4-layer-shell-0 2>/dev/null ||
    fail "gtk4-layer-shell required (pacman -S gtk4-layer-shell | apt install libgtk4-layer-shell-dev)"

# ── build ─────────────────────────────────────────────────────────────────────
workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

say "fetching $REPO"
git clone --depth 1 "$REPO" "$workdir/src" >/dev/null 2>&1 || fail "clone failed"

say "building (first build compiles the GTK bindings — can take a few minutes)"
(cd "$workdir/src" && go build -o "$workdir/nz" .) 2>"$workdir/build.log" || {
    cat "$workdir/build.log" >&2
    fail "build failed"
}

# ── install ───────────────────────────────────────────────────────────────────
prefix="${NZ_PREFIX:-}"
if [[ -z $prefix ]]; then
    if [[ -w /usr/local/bin ]]; then
        prefix=/usr/local/bin
    elif command -v sudo >/dev/null && sudo -n true 2>/dev/null; then
        prefix=/usr/local/bin
    else
        prefix="$HOME/.local/bin"
    fi
fi

mkdir -p "$prefix" 2>/dev/null || true
if [[ -w $prefix ]]; then
    install -m755 "$workdir/nz" "$prefix/nz"
else
    say "installing to $prefix needs sudo"
    sudo install -m755 "$workdir/nz" "$prefix/nz"
fi

say "installed: $prefix/nz"
case ":$PATH:" in
*":$prefix:"*) ;;
*) say "note: $prefix is not in your PATH" ;;
esac

cat <<'EOF'

next steps:
  1. bind it in hyprland:   bind = SUPER, SPACE, exec, nz bar
  2. run it once — default config is written to ~/.config/neZha/
  3. make it yours:         ~/.config/neZha/catalog.toml

EOF
