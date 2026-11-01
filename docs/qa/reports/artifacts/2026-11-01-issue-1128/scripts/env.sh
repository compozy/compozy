: "${LAB:?Set LAB to the absolute path of an isolated QA lab}"
: "${WS:?Set WS to that lab workspace ID}"
for v in $(env | grep -o '^COMPOZY_[A-Z_]*'); do unset $v; done
unset CLAUDECODE CLAUDE_CODE_SSE_PORT
source "$LAB/qa-artifacts/qa/bootstrap.env" || return 1
export PATH="$LAB/bin:$PATH"
export QA=$QA_OUTPUT_PATH
EV=$QA/evidence
SEQ_FILE=$EV/.seq
# step <surface> <name> <cmd...>: run, save output + exit code, append a journey-log row
step() {
  local surface=$1 name=$2; shift 2
  local n; n=$(( $(cat $SEQ_FILE 2>/dev/null || echo 0) + 1 )); echo $n > $SEQ_FILE
  local f; f=$(printf "%s/%03d-%s.out" "$EV" "$n" "$name")
  local code=0
  { echo "\$ $*"; "$@" || code=$?; printf "\n[exit %s]\n" "$code"; } > "$f" 2>&1
  local command_json
  command_json=$(printf '%s' "$*" | jq -Rs .) || return 1
  printf '{"seq":%d,"ts":"%s","surface":"%s","action":"%s","command":%s,"exit":%s,"evidence":"evidence/%s"}\n' \
    "$n" "$(date -u +%FT%TZ)" "$surface" "$name" "$command_json" "$code" "$(basename $f)" >> "$JOURNEY_LOG" || return 1
  cat "$f"
  return "$code"
}
# last_json: print JSON body of the most recent evidence file (drops the "$ cmd" and "[exit N]" lines)
last_json() { sed '1d;$d' "$(ls -t $EV/*.out | head -1)"; }
# dbmon: durable monitor rows (owner_binding redacted to its length)
dbmon() { sqlite3 -readonly -header -column $COMPOZY_HOME/compozy.db "select id, session_id, kind, terminal_id, status, reason, exit_code, deadline_at, fired_at, length(owner_binding) as binding_len, version from session_monitors order by created_at"; }
