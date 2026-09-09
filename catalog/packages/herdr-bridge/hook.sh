#!/bin/sh
# Spool atomically and detach quickly so short-lived hooks survive daemon teardown.
set -u
umask 077
dir="${XDG_STATE_HOME:-$HOME/.local/state}/herdr-bridge/spool"
mkdir -p "$dir" && chmod 700 "$dir" || exit 0
f="$dir/$$-$(od -An -N4 -tu4 /dev/urandom | tr -d ' ').json"
cat > "$f.tmp" && mv "$f.tmp" "$f"
here="$(cd "$(dirname "$0")" && pwd)"
nohup "$here/bridge.py" --drain >/dev/null 2>&1 &
exit 0
