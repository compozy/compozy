#!/bin/bash
source "$(dirname "${BASH_SOURCE[0]}")/env.sh" || exit 1
cd "$LAB" || exit 1
EV=$QA/evidence-external-wait; SEQ_FILE=$EV/.seq; : > $SEQ_FILE
step cli session-new-ext compozy session new --agent mon-ext --workspace $LAB -o json >/dev/null || exit 1; S1=$(last_json | jq -r .id); echo $S1 > $QA/s1.id
step cli prompt-register compozy session prompt $S1 "[watch-ext] please watch the external condition" >/dev/null || exit 1
echo "registered $(date -u +%FT%TZ)" > $EV/ext-registered.txt
step api monitors-after-register curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S1/monitors" >/dev/null || exit 1
step runtime db-monitors-after-register dbmon >/dev/null || exit 1
sleep 200
echo external-ready > $QA/ext-marker; echo "marker written $(date -u +%FT%TZ)" > $EV/marker-flip.txt
for i in $(seq 1 30); do [ "$(sqlite3 -readonly $COMPOZY_HOME/compozy.db "select status from session_monitors where session_id='$S1'")" = fired ] && break; sleep 2; done
sleep 3
step api monitors-after-wake curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S1/monitors" >/dev/null || exit 1
step cli session-events-after-wake compozy session events $S1 -o json >/dev/null || exit 1
step runtime db-monitors-after-wake dbmon >/dev/null || exit 1
echo finished1
