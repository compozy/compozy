#!/bin/bash
source "$(dirname "${BASH_SOURCE[0]}")/env.sh" || exit 1
cd "$LAB" || exit 1
mkses() { step cli "session-new-$1" compozy session new --agent "$1" --workspace $LAB -o json >/dev/null || exit 1; last_json | jq -r .id; }
S2=$(mkses mon-restart) || exit 1; S3=$(mkses mon-stopped) || exit 1; echo $S2 > $QA/s2.id; echo $S3 > $QA/s3.id
step cli prompt-register-s2 compozy session prompt $S2 "[watch] register restart monitors" >/dev/null || exit 1
step cli prompt-register-s3 compozy session prompt $S3 "[watch] register stopped-owner monitor" >/dev/null || exit 1
T0=$(date -u +%FT%TZ); echo "registered at $T0" > $EV/phase2-t0.txt
step api monitors-s2-before-restart curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null || exit 1
step api monitors-s3-before-stop curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S3/monitors" >/dev/null || exit 1
step runtime db-monitors-before-restart dbmon >/dev/null || exit 1
step cli session-stop-s3-operator compozy session stop $S3 >/dev/null || exit 1
step runtime db-monitors-after-operator-stop dbmon >/dev/null || exit 1
step api daemon-restart curl --fail --silent --show-error -X POST --unix-socket $COMPOZY_UDS_PATH "http://unix/api/settings/actions/restart" >/dev/null || exit 1
OP=$(last_json | jq -er '.operation_id') || exit 1
READY=false
for i in $(seq 1 60); do
  response=$(curl --fail --silent --show-error --max-time 3 --unix-socket "$COMPOZY_UDS_PATH" "http://unix/api/settings/actions/restart/$OP" 2>/dev/null) || response='{}'
  status=$(printf '%s' "$response" | jq -r '.status // "unavailable"')
  if [ "$status" = ready ]; then
    printf '%s\n' "$response" > "$EV/restart-ready.json"
    READY=true
    break
  fi
  if [ "$status" = failed ]; then
    printf '%s\n' "$response" > "$EV/restart-failed.json"
    exit 1
  fi
  sleep 2
done
if [ "$READY" != true ]; then
  date -u +%FT%TZ > "$EV/restart-timeout.txt"
  exit 1
fi
date -u +%FT%TZ > $EV/phase2-daemon-back.txt
step runtime db-monitors-after-restart dbmon >/dev/null || exit 1
step cli session-list-after-restart compozy session list --workspace $LAB -o json >/dev/null || exit 1
sleep 8
step api monitors-s2-after-restart curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null || exit 1
step cli session-events-s2-after-restart compozy session events $S2 -o json >/dev/null || exit 1
echo phase2-boot-done
