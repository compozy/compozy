#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' 1>&2
Usage: print-session-command.sh --feature-name <name> --repo-root <path> --kind <kind> [--exec]

Kinds:
  codex-launch
  claude-launch
  prd
  techspec
  tasks
  validate
  start
  prd-path
  techspec-path
  tasks-dir
EOF
    exit 1
}

feature_name=""
repo_root=""
kind=""
exec_mode=0

while [ $# -gt 0 ]; do
    case "$1" in
        --feature-name)
            [ $# -ge 2 ] || usage
            feature_name=$2
            shift 2
            ;;
        --repo-root)
            [ $# -ge 2 ] || usage
            repo_root=$2
            shift 2
            ;;
        --kind)
            [ $# -ge 2 ] || usage
            kind=$2
            shift 2
            ;;
        --exec)
            exec_mode=1
            shift
            ;;
        *)
            usage
            ;;
    esac
done

[ -n "$feature_name" ] || usage
[ -n "$repo_root" ] || usage
[ -n "$kind" ] || usage

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
plan_script=$script_dir/render-session-plan.py

eval "$(
    python3 "$plan_script" \
        --feature-name "$feature_name" \
        --repo-root "$repo_root"
)"

case "$kind" in
    codex-launch)
        value=$CODEX_LAUNCH
        ;;
    claude-launch)
        value=$CLAUDE_LAUNCH
        ;;
    prd)
        value=$PRD_COMMAND
        ;;
    techspec)
        value=$TECHSPEC_COMMAND
        ;;
    tasks)
        value=$TASKS_COMMAND
        ;;
    validate)
        value=$VALIDATE_COMMAND
        ;;
    start)
        value=$START_COMMAND
        ;;
    prd-path)
        value=$PRD_PATH
        ;;
    techspec-path)
        value=$TECHSPEC_PATH
        ;;
    tasks-dir)
        value=$TASKS_DIR
        ;;
    *)
        usage
        ;;
esac

if [ "$exec_mode" -eq 1 ]; then
    exec sh -lc "$value"
fi

printf '%s\n' "$value"
