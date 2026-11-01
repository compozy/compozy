#!/bin/bash
source <lab>/scripts/env.sh
cd $LAB; WS=ws_93d7060986409c74
mkses() { step cli "session-new-$1" compozy session new --agent "$1" --workspace $LAB -o json >/dev/null; last_json | jq -r .id; }
S2=$(mkses mon-restart); S3=$(mkses mon-stopped); echo $S2 > $QA/s2.id; echo $S3 > $QA/s3.id
step cli prompt-register-s2 compozy session prompt $S2 "[watch] register restart monitors" >/dev/null
step cli prompt-register-s3 compozy session prompt $S3 "[watch] register stopped-owner monitor" >/dev/null
T0=$(date -u +%FT%TZ); echo "registered at $T0" > $EV/phase2-t0.txt
step api monitors-s2-before-restart curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null
step api monitors-s3-before-stop curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S3/monitors" >/dev/null
step runtime db-monitors-before-restart dbmon >/dev/null
step cli session-stop-s3-operator compozy session stop $S3 >/dev/null
step runtime db-monitors-after-operator-stop dbmon >/dev/null
step api daemon-restart curl -s -X POST --unix-socket $COMPOZY_UDS_PATH "http://unix/api/settings/actions/restart" >/dev/null
for i in $(seq 1 60); do sleep 2; compozy session list --workspace $LAB -o json >/dev/null 2>&1 && [ -n "$(sqlite3 -readonly $COMPOZY_HOME/compozy.db 'select 1' 2>/dev/null)" ] && out=$(compozy daemon start 2>&1 | grep -c 'Status: *running'); [ "${out:-0}" -ge 1 ] && break; done
date -u +%FT%TZ > $EV/phase2-daemon-back.txt
step runtime db-monitors-after-restart dbmon >/dev/null
step cli session-list-after-restart compozy session list --workspace $LAB -o json >/dev/null
sleep 8
step api monitors-s2-after-restart curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null
step cli session-events-s2-after-restart compozy session events $S2 -o json >/dev/null
echo phase2-boot-done
