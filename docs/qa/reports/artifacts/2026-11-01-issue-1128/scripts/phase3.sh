#!/bin/bash
source <lab>/scripts/env.sh
cd $LAB; WS=ws_93d7060986409c74; S2=$(cat $QA/s2.id); S3=$(cat $QA/s3.id)
DL=$(sqlite3 -readonly $COMPOZY_HOME/compozy.db "select min(deadline_at) from session_monitors where session_id='$S2' and kind='timer'")
until [ "$(sqlite3 -readonly $COMPOZY_HOME/compozy.db "select status from session_monitors where session_id='$S2' and kind='timer'")" = "fired" ]; do sleep 3; done
sleep 3
step api monitors-s2-after-wakes curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null
step cli session-events-s2-after-wakes compozy session events $S2 -o json >/dev/null
step runtime db-monitors-final dbmon >/dev/null
step cli session-s3-state compozy session health $S3 >/dev/null
step cli session-events-s3-final compozy session events $S3 -o json >/dev/null
echo "$DL" > $EV/phase3-timer-deadline.txt; date -u +%FT%TZ > $EV/phase3-done.txt
echo finished3
