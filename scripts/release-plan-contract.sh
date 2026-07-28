#!/usr/bin/env bash
set -euo pipefail

readonly PR_RELEASE_MODULE="${1:-github.com/compozy/releasepr@v0.0.24}"
readonly CONTRACT_VERSION="0.3.0-beta.1"
readonly CONTRACT_TAG="v${CONTRACT_VERSION}"
export PR_RELEASE_LOG_LEVEL=error

fail() {
  printf 'release plan contract: %s\n' "$*" >&2
  exit 1
}

assert_line() {
  local output="$1"
  local expected="$2"
  grep -Fqx "$expected" <<<"$output" || fail "missing output line: ${expected}"
}

expect_failure() {
  local expected="$1"
  shift
  local output
  if output="$("$@" 2>&1)"; then
    fail "command unexpectedly succeeded: $*"
  fi
  grep -Fq "$expected" <<<"$output" || fail "failure did not contain '${expected}': ${output}"
}

run_plan() {
  go run "$PR_RELEASE_MODULE" plan "$@" --format github
}

assert_plan() {
  local ref="$1"
  local version="$2"
  local channel="$3"
  local github_prerelease="$4"
  local github_make_latest="$5"
  local npm_tag="$6"
  local homebrew_skip_upload="$7"
  local output
  output="$(run_plan --ref "$ref" --version "$version" --channel "$channel")"

  local commit
  commit="$(git rev-parse HEAD)"
  assert_line "$output" "release_ref=${ref}"
  assert_line "$output" "release_commit=${commit}"
  assert_line "$output" "release_version=${version}"
  assert_line "$output" "release_tag=v${version}"
  assert_line "$output" "release_channel=${channel}"
  assert_line "$output" "github_prerelease=${github_prerelease}"
  assert_line "$output" "github_make_latest=${github_make_latest}"
  assert_line "$output" "npm_tag=${npm_tag}"
  assert_line "$output" "homebrew_skip_upload=${homebrew_skip_upload}"

  local output_lines
  output_lines="$(wc -l <<<"$output" | tr -d ' ')"
  [[ "$output_lines" == "9" ]] || fail "planner emitted ${output_lines} lines, want 9"
}

for command in git go; do
  command -v "$command" >/dev/null 2>&1 || fail "${command} is required"
done

contract_root="$(mktemp -d "${TMPDIR:-/tmp}/compozy-release-plan.XXXXXX")"
trap 'rm -rf "$contract_root"' EXIT INT TERM

remote_dir="${contract_root}/origin.git"
repo_dir="${contract_root}/repo"
git init --bare "$remote_dir" >/dev/null
git init --initial-branch=main "$repo_dir" >/dev/null
cd "$repo_dir"
git config user.name "Compozy Release Contract"
git config user.email "release-contract@compozy.com"
git remote add origin "$remote_dir"

printf 'first\n' > contract.txt
git add contract.txt
git commit -m "test: seed release contract" >/dev/null
printf 'second\n' >> contract.txt
git add contract.txt
git commit -m "test: advance release candidate" >/dev/null
git push --set-upstream origin main >/dev/null

before_head="$(git rev-parse HEAD)"
before_status="$(git status --porcelain)"
assert_plan HEAD "$CONTRACT_VERSION" beta true false beta true
[[ "$(git rev-parse HEAD)" == "$before_head" ]] || fail "planner changed HEAD"
[[ "$(git status --porcelain)" == "$before_status" ]] || fail "planner changed the worktree"
[[ -z "$(git tag --list "$CONTRACT_TAG")" ]] || fail "planner created the release tag"

expect_failure "release version must not start with v" \
  run_plan --ref HEAD --version "$CONTRACT_TAG" --channel beta
expect_failure "does not match HEAD" \
  run_plan --ref HEAD~1 --version "$CONTRACT_VERSION" --channel beta

git tag "$CONTRACT_TAG"
expect_failure "release tag \"${CONTRACT_TAG}\" already exists" \
  run_plan --ref HEAD --version "$CONTRACT_VERSION" --channel beta
git tag --delete "$CONTRACT_TAG" >/dev/null

git tag "$CONTRACT_TAG"
git push origin "refs/tags/${CONTRACT_TAG}" >/dev/null
git tag --delete "$CONTRACT_TAG" >/dev/null
expect_failure "release tag \"${CONTRACT_TAG}\" already exists" \
  run_plan --ref HEAD --version "$CONTRACT_VERSION" --channel beta

assert_plan HEAD 9.9.9 stable false true latest false
assert_plan HEAD 0.2.999 legacy false true latest false

printf 'release plan contract: PASS (%s)\n' "$PR_RELEASE_MODULE"
