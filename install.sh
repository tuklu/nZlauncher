#!/usr/bin/env bash
set -euo pipefail

# nZlauncher installer
#   curl -fsSL https://raw.githubusercontent.com/tuklu/nZlauncher/dev/install.sh | bash
#
# Tries the prebuilt release first (fast, no Go needed); falls back to
# building from source. On Arch, prefer packaging/arch/PKGBUILD.
#
# Overrides:
#   NZ_REPO    git url or local path to build from (default: github.com/tuklu/nZlauncher)
#   NZ_PREFIX  install dir (default: /usr/local/bin)
#   NZ_BUILD   set to 1 to skip the prebuilt and always build from source

REPO="${NZ_REPO:-https://github.com/tuklu/nZlauncher}"
RELEASE_URL="https://github.com/tuklu/nZlauncher/releases/latest/download"

say()  { printf '\033[1m[nzlauncher]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[nzlauncher]\033[0m %s\n' "$*" >&2; exit 1; }

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
binary=""

# ── prebuilt ──────────────────────────────────────────────────────────────────
# Two binaries: nzlauncher (static, no GTK) and nzlauncher-gtk (the GTK face).
if [[ ${NZ_BUILD:-0} != 1 && $(uname -sm) == "Linux x86_64" ]] && command -v curl >/dev/null; then
    say "trying prebuilt release"
    if curl -fsSL -o "$workdir/nzlauncher" "$RELEASE_URL/nzlauncher-linux-amd64" 2>/dev/null &&
       curl -fsSL -o "$workdir/nzlauncher-gtk" "$RELEASE_URL/nzlauncher-gtk-linux-amd64" 2>/dev/null; then
        chmod +x "$workdir/nzlauncher" "$workdir/nzlauncher-gtk"
        # the GTK face would open a window if run, so check its libraries instead
        if ldd "$workdir/nzlauncher-gtk" 2>/dev/null | grep -q 'not found'; then
            say "prebuilt GTK face is missing libraries here (gtk4, gtk4-layer-shell?), building from source"
        else
            binary=ok
            say "prebuilt binaries work"
        fi
    else
        say "no prebuilt available, building from source"
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

    say "building (first build compiles the GTK bindings, can take a few minutes)"
    (cd "$workdir/src" &&
        CGO_ENABLED=0 go build -ldflags "-s -w" -o "$workdir/nzlauncher" ./cmd/nzlauncher &&
        go build -ldflags "-s -w" -o "$workdir/nzlauncher-gtk" ./cmd/nzlauncher-gtk) 2>"$workdir/build.log" || {
        cat "$workdir/build.log" >&2
        fail "build failed"
    }
    binary=ok
fi

# ── install ───────────────────────────────────────────────────────────────────
prefix="${NZ_PREFIX:-/usr/local/bin}"

mkdir -p "$prefix" 2>/dev/null || true
if [[ -w $prefix ]]; then
    install -m755 "$workdir/nzlauncher" "$workdir/nzlauncher-gtk" "$prefix/"
else
    say "installing to $prefix needs sudo"
    sudo install -m755 "$workdir/nzlauncher" "$workdir/nzlauncher-gtk" "$prefix/"
fi

say "installed: $prefix/nzlauncher and $prefix/nzlauncher-gtk"
case ":$PATH:" in
*":$prefix:"*) ;;
*) say "note: $prefix is not in your PATH" ;;
esac

cat <<'EOF'

next steps:
  1. bind it in hyprland:   bind = SUPER, SPACE, exec, nzlauncher bar
  2. run it once, default config is written to ~/.config/nzlauncher/
  3. make it yours:         ~/.config/nzlauncher/catalog.toml
  4. shorter to type:       alias nz=nzlauncher

EOF
