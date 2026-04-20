#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' 1>&2
Usage: run-compozy-start.sh --feature-name <name> --repo-root <path>
EOF
    exit 1
}

feature_name=""
repo_root=""

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
        *)
            usage
            ;;
    esac
done

[ -n "$feature_name" ] || usage
[ -n "$repo_root" ] || usage

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
exec "$script_dir/print-session-command.sh" \
    --feature-name "$feature_name" \
    --repo-root "$repo_root" \
    --kind start \
    --exec
