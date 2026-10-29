#!/usr/bin/env bash
# Runs the compositor tests against a real sway, headless: no screen, no
# input devices, software rendering. Needs sway and foot. Arguments go to
# go test.
set -euo pipefail

runtime=$(mktemp -d)
chmod 700 "$runtime"
export XDG_RUNTIME_DIR=$runtime
unset HYPRLAND_INSTANCE_SIGNATURE NIRI_SOCKET WAYLAND_DISPLAY

WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1 \
    sway --config "$(dirname "$0")/config" >"$runtime/sway.log" 2>&1 &
sway_pid=$!
trap 'kill $sway_pid 2>/dev/null; rm -rf "$runtime"' EXIT

for _ in $(seq 100); do
    sock=$(ls "$runtime"/sway-ipc.*.sock 2>/dev/null | head -1 || true)
    [[ -n $sock ]] && break
    sleep 0.1
done
if [[ -z ${sock:-} ]]; then
    echo "sway did not start:" >&2
    cat "$runtime/sway.log" >&2
    exit 1
fi
export SWAYSOCK=$sock
export WAYLAND_DISPLAY=$(basename "$(ls "$runtime"/wayland-* | command grep -v lock | head -1)")

go test -tags swaylive -count=1 -timeout 60s "$@"
