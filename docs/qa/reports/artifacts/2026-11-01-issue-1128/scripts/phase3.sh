#!/bin/bash
source "$(dirname "${BASH_SOURCE[0]}")/env.sh" || exit 1
cd "$LAB" || exit 1; S2=$(cat $QA/s2.id); S3=$(cat $QA/s3.id)
DL=$(sqlite3 -readonly $COMPOZY_HOME/compozy.db "select min(deadline_at) from session_monitors where session_id='$S2' and kind='timer'")
WAIT_SECONDS=${MONITOR_WAIT_SECONDS:-1800}
case "$WAIT_SECONDS" in ''|*[!0-9]*) echo "MONITOR_WAIT_SECONDS must be a positive integer" >&2; exit 1;; esac
[ "$WAIT_SECONDS" -gt 0 ] || exit 1
END=$((SECONDS + WAIT_SECONDS))
FIRED=false
while [ "$SECONDS" -lt "$END" ]; do
  status=$(sqlite3 -readonly "$COMPOZY_HOME/compozy.db" "select status from session_monitors where session_id='$S2' and kind='timer'") || break
  if [ "$status" = fired ]; then FIRED=true; break; fi
  sleep 3
done
if [ "$FIRED" != true ]; then date -u +%FT%TZ > "$EV/phase3-timeout.txt"; fi
sleep 3
step api monitors-s2-after-wakes curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null || exit 1
step cli session-events-s2-after-wakes compozy session events $S2 -o json >/dev/null || exit 1
step runtime db-monitors-final dbmon >/dev/null || exit 1
step cli session-s3-state compozy session health $S3 >/dev/null || exit 1
step cli session-events-s3-final compozy session events $S3 -o json >/dev/null || exit 1
[ "$FIRED" = true ] || exit 1
echo "$DL" > $EV/phase3-timer-deadline.txt; date -u +%FT%TZ > $EV/phase3-done.txt
echo finished3
