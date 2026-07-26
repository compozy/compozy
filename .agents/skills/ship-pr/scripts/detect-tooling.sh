#!/usr/bin/env bash
# detect-tooling.sh — read-only probe of the current repo's ship-pr capabilities.
#
# Emits a single JSON object on stdout describing which optional tools and
# artifacts are available. The orchestrator branches off this JSON to decide
# whether each optional step in the ship-pr operating loop should run.
#
# Probes only — never mutates the working tree, never makes network calls.

set -euo pipefail

have() { command -v "$1" >/dev/null 2>&1; }
json_bool() { [ "$1" = "true" ] && printf 'true' || printf 'false'; }
json_str()  { printf '"%s"' "$(printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g')"; }

gh_present=$(have gh && echo true || echo false)
prrelease_present=$(have pr-release && echo true || echo false)
compozy_present=$(have compozy && echo true || echo false)
skeeper_present=$(have skeeper && echo true || echo false)

skeeper_configured=false
if [ "$skeeper_present" = "true" ] && [ -d .skeeper ] && [ -f skeeper.lock ]; then
    skeeper_configured=true
fi

commitlint_config=""
for candidate in \
    commitlint.config.js commitlint.config.cjs commitlint.config.mjs commitlint.config.ts \
    .commitlintrc .commitlintrc.js .commitlintrc.cjs .commitlintrc.mjs \
    .commitlintrc.json .commitlintrc.yaml .commitlintrc.yml; do
    if [ -f "$candidate" ]; then
        commitlint_config="$candidate"
        break
    fi
done
if [ -z "$commitlint_config" ] && [ -f package.json ]; then
    if grep -q '"commitlint"' package.json 2>/dev/null; then
        commitlint_config="package.json#commitlint"
    fi
fi

# QA artifact paths. Manual override first, then the living docs/qa tree, then the legacy per-round glob.
qa_paths_json="[]"
if [ -n "${QA_OUTPUT_PATH:-}" ] && [ -d "$QA_OUTPUT_PATH" ]; then
    qa_paths_json="[$(json_str "$QA_OUTPUT_PATH")]"
elif [ -d docs/qa ] && [ -n "$(ls docs/qa/reports 2>/dev/null)" ]; then
    qa_paths_json="[$(json_str "docs/qa")]"
elif [ -d .compozy/tasks ]; then
    qa_found=()
    while IFS= read -r dir; do
        [ -n "$dir" ] && qa_found+=("$dir")
    done < <(find .compozy/tasks -maxdepth 2 -type d -name qa 2>/dev/null | sort)
    if [ "${#qa_found[@]}" -gt 0 ]; then
        joined=""
        for d in "${qa_found[@]}"; do
            joined+="${joined:+,}$(json_str "$d")"
        done
        qa_paths_json="[$joined]"
    fi
fi

# Task slug candidates (immediate subdirs of .compozy/tasks, ignore underscores).
slug_json="[]"
if [ -d .compozy/tasks ]; then
    slugs=()
    while IFS= read -r dir; do
        name="$(basename "$dir")"
        case "$name" in
            _*|.*) continue ;;
        esac
        slugs+=("$name")
    done < <(find .compozy/tasks -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort)
    if [ "${#slugs[@]}" -gt 0 ]; then
        joined=""
        for s in "${slugs[@]}"; do
            joined+="${joined:+,}$(json_str "$s")"
        done
        slug_json="[$joined]"
    fi
fi

cat <<EOF
{
  "gh": $(json_bool "$gh_present"),
  "pr_release": $(json_bool "$prrelease_present"),
  "compozy": $(json_bool "$compozy_present"),
  "skeeper_installed": $(json_bool "$skeeper_present"),
  "skeeper_configured": $(json_bool "$skeeper_configured"),
  "commitlint_config": $(json_str "$commitlint_config"),
  "qa_output_paths": $qa_paths_json,
  "task_slug_candidates": $slug_json
}
EOF
