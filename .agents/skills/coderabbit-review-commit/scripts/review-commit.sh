#!/usr/bin/env bash
# review-commit.sh — Review EXACTLY ONE commit with the CodeRabbit CLI.
# Role: mutating (creates a throwaway git worktree + a log file; never alters your branch).
#
# The CodeRabbit CLI always diffs base -> HEAD (the tip), so a single non-tip
# commit C cannot be targeted directly. This script pins C in a detached git
# worktree (HEAD = C) and reviews base = C~1 -> HEAD = C. The current branch and
# working tree are untouched; the temp worktree is force-removed on every exit.
#
# Usage:  review-commit.sh [<commit-ish>]        # default: HEAD~1 (penultimate)
# Env overrides:
#   CR_REVIEW_OUT   log output dir   (default: <repo>/.coderabbit/reviews)
#   CR_CONFIG       coderabbit config (default: <repo>/.coderabbit.yaml, used only if it exists)
#   CR_REVIEW_TYPE  review scope     (default: committed)
# Background:  nohup review-commit.sh HEAD~1 >/dev/null 2>&1 &
set -euo pipefail

command -v coderabbit >/dev/null 2>&1 \
  || { echo "error: 'coderabbit' CLI not found in PATH — install & auth it (coderabbit auth login)" >&2; exit 127; }

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" \
  || { echo "error: not inside a git repository" >&2; exit 1; }

TARGET="${1:-HEAD~1}"
SHA="$(git -C "$REPO_ROOT" rev-parse --verify "${TARGET}^{commit}" 2>/dev/null)" \
  || { echo "error: '$TARGET' is not a valid commit" >&2; exit 1; }
SHORT="$(git -C "$REPO_ROOT" rev-parse --short "$SHA")"
PARENT="$(git -C "$REPO_ROOT" rev-parse --verify "${SHA}~1^{commit}" 2>/dev/null)" \
  || { echo "error: $SHORT is a root commit (no parent) — no base to diff against" >&2; exit 1; }

OUT_DIR="${CR_REVIEW_OUT:-$REPO_ROOT/.coderabbit/reviews}"
CONFIG="${CR_CONFIG:-$REPO_ROOT/.coderabbit.yaml}"
REVIEW_TYPE="${CR_REVIEW_TYPE:-committed}"
LOG="$OUT_DIR/$SHORT.log"
WT="${TMPDIR:-/tmp}/cr-review-$SHORT-$$"

cleanup() {
  cd "$REPO_ROOT" 2>/dev/null || true
  git worktree remove --force "$WT" 2>/dev/null || rm -rf "$WT"
}
trap cleanup EXIT

mkdir -p "$OUT_DIR"

CONFIG_DISPLAY="<none>"
[ -f "$CONFIG" ] && CONFIG_DISPLAY="$CONFIG"
{
  echo "=== CodeRabbit review — commit $SHORT ($SHA) ==="
  git -C "$REPO_ROOT" show --stat --oneline "$SHA" | tail -1
  echo "base (parent): $PARENT"
  echo "scope:         $REVIEW_TYPE"
  echo "config:        $CONFIG_DISPLAY"
  echo "worktree:      $WT"
  echo "started (UTC): $(date -u +%FT%TZ)"
  echo "----------------------------------------"
} | tee "$LOG"

# Isolated detached worktree pinned at the target commit.
git -C "$REPO_ROOT" worktree add --detach "$WT" "$SHA" >>"$LOG" 2>&1

# Review precisely this commit: base = parent, tip = HEAD (= target commit).
cd "$WT"
rc=0
if [ -f "$CONFIG" ]; then
  coderabbit review --type "$REVIEW_TYPE" --base-commit "$PARENT" --plain --config "$CONFIG" 2>&1 | tee -a "$LOG" || rc=$?
else
  coderabbit review --type "$REVIEW_TYPE" --base-commit "$PARENT" --plain 2>&1 | tee -a "$LOG" || rc=$?
fi

echo "=== finished (UTC): $(date -u +%FT%TZ) — exit $rc — log: $LOG ===" | tee -a "$LOG"
if [ "$rc" -eq 0 ]; then
  echo "review complete: $LOG"
else
  echo "review exited $rc (see $LOG)" >&2
fi
exit "$rc"
