#!/usr/bin/env sh
set -eu

root="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"

make -C "$root" build

DEV_HOME="${RUNLOOP_DEV_HOME:-$root/.runloop-dev-home}"
mkdir -p "$DEV_HOME"

if [ ! -f "$DEV_HOME/.config/runloop/config.yaml" ]; then
  HOME="$DEV_HOME" "$root/bin/runloop" init
fi

exec env HOME="$DEV_HOME" "$root/bin/runloopd" "$@"
