#!/usr/bin/env bash
set -euo pipefail

# nZlauncher installer
#   curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/main/install.sh | bash
#
# Tries the prebuilt release first (fast, no Go needed); falls back to
# building from source.
#
# Overrides:
#   NZ_REPO    git url or local path to build from (default: github.com/tuklu/nZlauncher)
#   NZ_PREFIX  install dir (default: /usr/local/bin, falls back to ~/.local/bin)
#   NZ_BUILD   set to 1 to skip the prebuilt and always build from source

REPO="${NZ_REPO:-https://github.com/tuklu/nZlauncher}"
RELEASE_URL="https://github.com/tuklu/nZlauncher/releases/latest/download/nz-linux-amd64"

say()  { printf '\033[1m[nz]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[nz]\033[0m %s\n' "$*" >&2; exit 1; }

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
binary=""

# ── prebuilt ──────────────────────────────────────────────────────────────────
if [[ ${NZ_BUILD:-0} != 1 && $(uname -sm) == "Linux x86_64" ]] && command -v curl >/dev/null; then
    say "trying prebuilt release"
    if curl -fsSL -o "$workdir/nz" "$RELEASE_URL" 2>/dev/null; then
        chmod +x "$workdir/nz"
        # exits 2 with usage when healthy; missing runtime libs fail differently
        if "$workdir/nz" >/dev/null 2>&1 || [[ $? == 2 ]]; then
            binary="$workdir/nz"
            say "prebuilt binary works"
        else
            say "prebuilt won't run here (missing gtk4/gtk4-layer-shell runtime?) — building from source"
        fi
    else
        say "no prebuilt available — building from source"
    fi
fi

# ── source build ──────────────────────────────────────────────────────────────
if [[ -z $binary ]]; then
    command -v go >/dev/null  || fail "go is required (pacman -S go | apt install golang)"
    command -v git >/dev/null || fail "git is required"
    command -v pkg-config >/dev/null || fail "pkg-config is required"
    pkg-config --exists gtk4 2>/dev/null ||
        fail "gtk4 dev files required (pacman -S gtk4 | apt install libgtk-4-dev)"
    pkg-config --exists gtk4-layer-shell-0 2>/dev/null ||
        fail "gtk4-layer-shell required (pacman -S gtk4-layer-shell | apt install libgtk4-layer-shell-dev)"

    say "fetching $REPO"
    git clone --depth 1 "$REPO" "$workdir/src" >/dev/null 2>&1 || fail "clone failed"

    say "building (first build compiles the GTK bindings — can take a few minutes)"
    (cd "$workdir/src" && go build -ldflags "-s -w" -o "$workdir/nz" ./cmd/nz) 2>"$workdir/build.log" || {
        cat "$workdir/build.log" >&2
        fail "build failed"
    }
    binary="$workdir/nz"
fi

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
    install -m755 "$binary" "$prefix/nz"
else
    say "installing to $prefix needs sudo"
    sudo install -m755 "$binary" "$prefix/nz"
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
