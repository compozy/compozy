#!/bin/sh
# SPDX-License-Identifier: FSL-1.1-MIT

set -eu

export COMPOZY_CLOUD_URL=$(cat /etc/compozy-fixture-cloud)
printf '%s\n' "${SSH_ORIGINAL_COMMAND:-}" >> "$HOME/fixture-commands"
case "${SSH_ORIGINAL_COMMAND:-}" in
    *' service install '* )
        if test -f "$HOME/fixture-drop-service"; then
            rm "$HOME/fixture-drop-service"
            kill -TERM "$PPID"
            exit 255
        fi
        ;;
    *' remote link --token-stdin '* )
        if test -f "$HOME/fixture-drop-link"; then
            rm "$HOME/fixture-drop-link"
            /bin/sh -c "$SSH_ORIGINAL_COMMAND" >/dev/null
            kill -TERM "$PPID"
            exit 255
        fi
        ;;
esac
exec /bin/sh -c "${SSH_ORIGINAL_COMMAND:-:}"
