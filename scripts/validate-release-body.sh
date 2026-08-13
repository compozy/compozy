#!/usr/bin/env bash
set -euo pipefail

readonly RELEASE_TAG="${1:-}"
readonly RELEASE_GIT_RANGE="${2:-}"
readonly RELEASE_INITIAL="${3:-}"
readonly RELEASE_BODY_PATH="${4:-}"

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

for command in git grep; do
  command -v "$command" >/dev/null 2>&1 || fail "${command} is required"
done

[[ -n "$RELEASE_TAG" ]] || fail "release tag is required"
[[ -s "$RELEASE_BODY_PATH" ]] || fail "release body is missing or empty"

commit_count=0
case "$RELEASE_INITIAL" in
  true)
    [[ -z "$RELEASE_GIT_RANGE" ]] || fail "initial release cannot include a Git range"
    ;;
  false)
    [[ -n "$RELEASE_GIT_RANGE" ]] || fail "non-initial release must include a Git range"
    if ! commit_count="$(git rev-list --count "$RELEASE_GIT_RANGE")"; then
      fail "release Git range does not resolve: ${RELEASE_GIT_RANGE}"
    fi
    [[ "$commit_count" =~ ^[0-9]+$ ]] || fail "release Git range produced an invalid commit count"
    ((commit_count > 0)) || fail "release Git range contains no commits: ${RELEASE_GIT_RANGE}"
    ;;
  *)
    fail "release initial flag must be true or false"
    ;;
esac

IFS= read -r first_line <"$RELEASE_BODY_PATH" || fail "release body has no heading"
version="${RELEASE_TAG#v}"
[[ "$first_line" == "## ${version} - "* ]] ||
  fail "release body heading '${first_line}' does not match ${RELEASE_TAG}"
grep -Eq '^- .+' "$RELEASE_BODY_PATH" || fail "release body contains no changelog entries"

printf 'release body validation: PASS (%s, %d commits)\n' "$RELEASE_TAG" "$commit_count"
