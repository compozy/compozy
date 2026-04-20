#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)

exec "$script_dir/print-session-command.sh" \
    --kind claude-launch \
    --exec \
    "$@"
