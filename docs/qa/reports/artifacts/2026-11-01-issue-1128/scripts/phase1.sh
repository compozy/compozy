#!/bin/bash
source <lab>/scripts/env.sh
cd $LAB; WS=ws_93d7060986409c74
EV=$QA/evidence-external-wait; SEQ_FILE=$EV/.seq; : > $SEQ_FILE
step cli session-new-ext compozy session new --agent mon-ext --workspace $LAB -o json >/dev/null; S1=$(last_json | jq -r .id); echo $S1 > $QA/s1.id
step cli prompt-register compozy session prompt $S1 "[watch-ext] please watch the external condition" >/dev/null
echo "registered $(date -u +%FT%TZ)" > $EV/ext-registered.txt
step api monitors-after-register curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S1/monitors" >/dev/null
step runtime db-monitors-after-register dbmon >/dev/null
sleep 200
echo external-ready > $QA/ext-marker; echo "marker written $(date -u +%FT%TZ)" > $EV/marker-flip.txt
for i in $(seq 1 30); do [ "$(sqlite3 -readonly $COMPOZY_HOME/compozy.db "select status from session_monitors where session_id='$S1'")" = fired ] && break; sleep 2; done
sleep 3
step api monitors-after-wake curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S1/monitors" >/dev/null
step cli session-events-after-wake compozy session events $S1 -o json >/dev/null
step runtime db-monitors-after-wake dbmon >/dev/null
echo finished1
