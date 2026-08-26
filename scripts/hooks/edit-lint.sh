#!/usr/bin/env bash
# Edit-time fmt + lint dispatcher shared by the Claude Code, Codex, and Cursor
# hooks. Parses the host payload once, then routes the edited files by language:
#   *.go                                -> go-edit-lint.sh (golangci-lint)
#   js/ts + css/html/json/jsonc/yaml/md -> ts-edit-lint.sh (oxfmt + oxlint)
#
# Payload shapes (stdin JSON):
#   Claude Code PostToolUse  -> .tool_input.file_path
#   Codex PostToolUse        -> .tool_input.file_path, or the raw patch in
#                               .tool_input.command ("*** Update File: <path>")
#   Cursor afterFileEdit     -> top-level .file_path (no .tool_input)
#
# Modes (EDIT_LINT_MODE overrides; default derives from the payload):
#   check -> formatters auto-apply and linter findings exit 2 on stderr
#            (Claude Code and Codex feed exit-2 stderr back to the model).
#   fix   -> formatters auto-apply only; Cursor's afterFileEdit has no
#            feedback channel, so linter findings would be unread noise.
#
# Deliberately fail-open: missing tooling, foreign paths, or a runner crash
# skip the check (exit 0) — `make gate` and PR CI own enforcement.
set -uo pipefail

payload=$(cat)
command -v jq >/dev/null 2>&1 || exit 0

hook_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd) || exit 0

mode="${EDIT_LINT_MODE:-}"
if [ -z "$mode" ]; then
  if jq -e '.tool_input | type == "object"' <<<"$payload" >/dev/null 2>&1; then
    mode="check"
  else
    mode="fix"
  fi
fi

candidates=$(jq -r '
  [
    (.tool_input.file_path? // empty),
    (.tool_input.path? // empty),
    (.file_path? // empty)
  ] | .[]' <<<"$payload" 2>/dev/null)
if jq -e '.tool_name == "apply_patch" and (.tool_input.command | type == "string")' <<<"$payload" >/dev/null 2>&1; then
  patch_paths=$(jq -r '.tool_input.command' <<<"$payload" |
    sed -n -e 's/^\*\*\* Update File: //p' -e 's/^\*\*\* Add File: //p')
  candidates=$(printf '%s\n%s' "$candidates" "$patch_paths")
fi
[ -n "$(printf '%s' "$candidates" | tr -d '[:space:]')" ] || exit 0

root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$root" ]; then
  root=$(jq -r '.cwd // .workspace_roots[0]? // empty' <<<"$payload" 2>/dev/null)
  [ -d "$root" ] || root=""
fi
if [ -z "$root" ]; then
  first=$(printf '%s\n' "$candidates" | sed '/^$/d' | head -n 1)
  probe_dir=$(dirname "$first")
  [ -d "$probe_dir" ] || probe_dir="$PWD"
  root=$(git -C "$probe_dir" rev-parse --show-toplevel 2>/dev/null) || root="$PWD"
fi
cd "$root" 2>/dev/null || exit 0

go_files=()
ts_files=()
while IFS= read -r candidate; do
  [ -n "$candidate" ] || continue
  case "$candidate" in
    /*) rel="${candidate#"$root"/}" ;;
    *) rel="$candidate" ;;
  esac
  case "$rel" in
    /*) continue ;;
  esac
  [ -f "$rel" ] || continue
  case "$rel" in
    *.go)
      case " ${go_files[*]-} " in
        *" $rel "*) ;;
        *) go_files+=("$rel") ;;
      esac
      ;;
    *.js | *.jsx | *.ts | *.tsx | *.mjs | *.cjs | *.mts | *.cts | *.css | *.html | *.json | *.jsonc | *.yaml | *.yml | *.md)
      case " ${ts_files[*]-} " in
        *" $rel "*) ;;
        *) ts_files+=("$rel") ;;
      esac
      ;;
  esac
done <<<"$candidates"

issues=""
if [ "${#go_files[@]}" -gt 0 ]; then
  go_out=$("$hook_dir/go-edit-lint.sh" --mode "$mode" "${go_files[@]}" 2>/dev/null)
  if [ $? -eq 2 ] && [ -n "$go_out" ]; then
    issues="$go_out
"
  fi
fi
if [ "${#ts_files[@]}" -gt 0 ]; then
  ts_out=$("$hook_dir/ts-edit-lint.sh" --mode "$mode" "${ts_files[@]}" 2>/dev/null)
  if [ $? -eq 2 ] && [ -n "$ts_out" ]; then
    issues="${issues}${ts_out}
"
  fi
fi

if [ -n "$issues" ]; then
  printf 'edit-time check (fast tier; the full gate still owns type-aware checks):\n%s' "$issues" | head -c 4000 >&2
  exit 2
fi
exit 0
