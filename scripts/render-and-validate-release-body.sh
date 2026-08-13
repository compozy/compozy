#!/usr/bin/env bash
set -euo pipefail

readonly PR_RELEASE_MODULE="${1:-}"
readonly RELEASE_TAG="${2:-}"
readonly RELEASE_GIT_RANGE="${3:-}"
readonly RELEASE_INITIAL="${4:-}"
readonly RELEASE_BODY_PATH="${5:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

[[ -n "$PR_RELEASE_MODULE" ]] || fail "releasepr module is required"
[[ -n "$RELEASE_TAG" ]] || fail "release tag is required"
[[ -n "$RELEASE_BODY_PATH" ]] || fail "release body output path is required"

release_body_args=(release-body --tag "$RELEASE_TAG")
case "$RELEASE_INITIAL" in
  true)
    [[ -z "$RELEASE_GIT_RANGE" ]] || fail "initial release cannot include a Git range"
    release_body_args+=(--initial)
    ;;
  false)
    [[ -n "$RELEASE_GIT_RANGE" ]] || fail "non-initial release must include a Git range"
    release_body_args+=(--range "$RELEASE_GIT_RANGE")
    ;;
  *)
    fail "release initial flag must be true or false"
    ;;
esac

go run "$PR_RELEASE_MODULE" "${release_body_args[@]}" >"$RELEASE_BODY_PATH"
bash "$SCRIPT_DIR/validate-release-body.sh" \
  "$RELEASE_TAG" \
  "$RELEASE_GIT_RANGE" \
  "$RELEASE_INITIAL" \
  "$RELEASE_BODY_PATH"
