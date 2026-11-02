#!/bin/bash
# Isolated session-monitor lab. Targets hold a turn until canceled; owners register monitors through the real native tool.
export LAB=/Users/pedronauck/dev/qa-labs/compozy-session-monitor-kind-20261102-150548-331198-lab
for v in $(env | grep -o '^COMPOZY_[A-Z_]*'); do unset $v; done
unset CLAUDECODE CLAUDE_CODE_SSE_PORT
source "$LAB/qa-artifacts/qa/bootstrap.env" || exit 1
mkdir -p "$LAB/bin" "$QA_OUTPUT_PATH/evidence"
/tmp/smk/compozy daemon stop >/dev/null 2>&1
sleep 2
rm -f "$LAB/bin/compozy" "$LAB/bin/acpmock-driver"
cp /tmp/smk/compozy /tmp/smk/acpmock-driver "$LAB/bin/"
export PATH="$LAB/bin:$PATH"
EV=$QA_OUTPUT_PATH/evidence
FX=$LAB/fixture.json
cd "$LAB" || exit 1
agent() {
  mkdir -p "$COMPOZY_HOME/agents/$1"
  cat > "$COMPOZY_HOME/agents/$1/AGENT.md" <<MD
---
name: $1
provider: claude
model: claude-sonnet-5-5
command: '$LAB/bin/acpmock-driver --fixture $FX --agent $1 --provider claude'
---
QA agent.
MD
}
agent mon-target; agent mon-owner
python3 - "$FX" <<'PY'
import json,sys
opts=[{"id":"model","name":"Model","current":"claude-sonnet-5-5","values":[{"value":"claude-sonnet-5-5","label":"Sonnet"},{"value":"sonnet","label":"Sonnet alias"}]}]
t=[{"name":"hold","match":{"user_text_contains":"[hold]"},"steps":[{"kind":"driver_control","driver_control":{"action":"block_until_cancel"}}]},
   {"name":"quick","match":{"user_text_contains":"[quick]"},"steps":[{"kind":"assistant","text":"quick done"}]}]
json.dump({"version":2,"agents":[
 {"name":"mon-target","provider":"claude","permissions":"approve-all","prompt":"t","config_options":opts,"turns":t},
 {"name":"mon-owner","provider":"claude","permissions":"approve-all","prompt":"o","config_options":opts,"turns":[]}]},open(sys.argv[1],"w"))
PY
compozy daemon start > "$EV/daemon-start.out" 2>&1
for i in $(seq 1 60); do compozy workspace list -o json >/dev/null 2>&1 && break; sleep 1; done
compozy workspace add "$LAB" -o json > "$EV/workspace-add.out" 2>&1
cat "$EV/workspace-add.out" | head -3
mk() { compozy session new --agent "$1" --workspace "$LAB" -o json | jq -r .id; }
T1=$(mk mon-target); T2=$(mk mon-target); T3=$(mk mon-target); T4=$(mk mon-target); T5=$(mk mon-target); T6=$(mk mon-target)
WS=$(compozy session status "$T1" -o json | jq -r '.workspace_id // .session.workspace_id')
echo "targets $T1 $T2 $T3 $T4 $T5 $T6 ws=$WS"
python3 - "$FX" "$T1" "$T2" "$T3" "$T4" "$T5" "$T6" <<'PY'
import json,sys
fx=sys.argv[1]; T=sys.argv[2:]
d=json.load(open(fx))
def turn(mark,raw):
    return {"name":mark,"match":{"user_text_contains":mark},"steps":[
      {"kind":"native_tool_call","tool_id":"compozy__monitor_create","tool_call_id":mark,"raw_input":raw},
      {"kind":"assistant","text":"registered "+mark}]}
o=d["agents"][1]
o["turns"]=[
 turn("[all]",{"kind":"session","description":"all workers","target_session_ids":[T[0],T[1]],"until":"all"}),
 turn("[any]",{"kind":"session","description":"first worker","target_session_ids":[T[2],T[3]],"until":"any"}),
 turn("[restart]",{"kind":"session","description":"across restart","target_session_ids":[T[4]]}),
 turn("[tmo]",{"kind":"session","description":"times out","target_session_ids":[T[5]],"timeout_seconds":3}),
 {"name":"wake","match":{"user_text_contains":"COMPOZY MONITOR"},"steps":[{"kind":"assistant","text":"woken"}]}]
json.dump(d,open(fx,"w"))
PY
mons() { curl -s --unix-socket "$COMPOZY_UDS_PATH" "http://unix/api/workspaces/$WS/sessions/$1/monitors?all_profiles=true" | jq -c '[.monitors[]|{monitor_id,kind,status,reason,until,targets}]'; }
badge() { compozy session status "$1" -o json | jq -r '.badge // .session.badge'; }
hold() { compozy session prompt "$1" "[hold] work" >/dev/null 2>&1 & p=$!; (sleep 2; kill $p 2>/dev/null) & }
until_status() { for i in $(seq 1 ${3:-40}); do mons "$1" | grep -q "\"status\":\"$2\"" && return 0; sleep 0.5; done; return 1; }
wakes() { compozy session events "$1" -o json 2>/dev/null | grep -o 'COMPOZY MONITOR - NOT USER INPUT' | wc -l | tr -d ' '; }
for t in $T1 $T2 $T3 $T4 $T5 $T6; do hold $t; done
sleep 3
echo "--- badges: T1=$(badge $T1) T3=$(badge $T3)"

echo "=== A all"
A=$(mk mon-owner); compozy session prompt "$A" "[all] watch" > "$EV/A-prompt.out" 2>&1
sleep 1; echo "A after register: $(mons $A) badge=$(badge $A)"
compozy session prompt-cancel "$T1" > /dev/null; sleep 3
echo "A after T1 settled ($(badge $T1)): $(mons $A) badge=$(badge $A)"
compozy session prompt-cancel "$T2" > /dev/null
until_status "$A" fired; echo "A after T2 settled: $(mons $A)"; sleep 2
echo "A wake text count: $(wakes $A); badge=$(badge $A)"
compozy session events "$A" -o json > "$EV/A-events.json" 2>&1

echo "=== B any + operator stop"
B=$(mk mon-owner); compozy session prompt "$B" "[any] watch" > /dev/null 2>&1
sleep 1; compozy session stop "$T3" > /dev/null 2>&1
until_status "$B" fired; echo "B: $(mons $B)"

echo "=== D timeout"
D=$(mk mon-owner); compozy session prompt "$D" "[tmo] watch" > /dev/null 2>&1
until_status "$D" fired; echo "D: $(mons $D)"

echo "=== C restart"
C=$(mk mon-owner); compozy session prompt "$C" "[restart] watch" > /dev/null 2>&1
sleep 1; echo "C before: $(mons $C) T5=$(badge $T5)"
OP=$(curl -s -X POST --unix-socket "$COMPOZY_UDS_PATH" http://unix/api/settings/actions/restart | jq -r .operation_id)
echo "restart op $OP"
for i in $(seq 1 90); do s=$(curl -s --max-time 3 --unix-socket "$COMPOZY_UDS_PATH" "http://unix/api/settings/actions/restart/$OP" 2>/dev/null | jq -r '.status // "unavailable"'); [ "$s" = ready ] && break; sleep 1; done
echo "restart status=$s"
sqlite3 -readonly -header -column "$COMPOZY_HOME/compozy.db" "select id,session_id,kind,status,reason,fired_at from session_monitors order by created_at" | tee "$EV/db-after-restart.txt"
sleep 8
echo "C after boot: $(mons $C) T5=$(badge $T5) C=$(badge $C) wakes=$(wakes $C)"
compozy session events "$C" -o json > "$EV/C-events.json" 2>&1
grep -h 'monitor\.' "$COMPOZY_HOME"/logs/*.log 2>/dev/null | tail -30 > "$EV/monitor-log.txt"
echo DONE
