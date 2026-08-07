#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

version="${1:-}"
npm_tag="${2:-}"
shift 2 || fail "usage: release-npm-readiness.sh <version> <npm-tag> <package> [package...]"

[[ -n "$version" ]] || fail "release version is required"
[[ "$npm_tag" =~ ^[a-z][a-z0-9._-]*$ ]] || fail "npm tag is invalid"
(($# > 0)) || fail "at least one npm package is required"

timeout_seconds="${NPM_RELEASE_READINESS_TIMEOUT_SECONDS:-600}"
poll_interval_seconds="${NPM_RELEASE_READINESS_INTERVAL_SECONDS:-10}"
[[ "$timeout_seconds" =~ ^[1-9][0-9]*$ ]] || fail "readiness timeout must be a positive integer"
[[ "$poll_interval_seconds" =~ ^[1-9][0-9]*$ ]] || fail "readiness interval must be a positive integer"

for required_command in node npm sleep; do
  command -v "$required_command" >/dev/null 2>&1 || fail "$required_command is required"
done

deadline=$((SECONDS + timeout_seconds))
attempt=0

while true; do
  attempt=$((attempt + 1))
  all_ready=true
  observations=()

  for package_name in "$@"; do
    if ! dist_tags="$(
      npm view "$package_name" dist-tags --json \
        --registry=https://registry.npmjs.org \
        --prefer-online \
        --fetch-retries=0 \
        --fetch-timeout=10000
    )"; then
      fail "npm registry query failed for ${package_name}"
    fi

    if evaluation="$(
      PACKAGE_NAME="$package_name" \
        DIST_TAGS_JSON="$dist_tags" \
        EXPECTED_TAG="$npm_tag" \
        RELEASE_VERSION="$version" \
        node <<'NODE'
const packageName = process.env.PACKAGE_NAME;
const expectedTag = process.env.EXPECTED_TAG;
const version = process.env.RELEASE_VERSION;
let tags;
try {
  tags = JSON.parse(process.env.DIST_TAGS_JSON || "");
} catch (error) {
  console.log(`invalid registry JSON for ${packageName}: ${error.message}`);
  process.exit(65);
}
if (!tags || Array.isArray(tags) || typeof tags !== "object") {
  console.log(`invalid registry dist-tags object for ${packageName}`);
  process.exit(65);
}
if (expectedTag === "beta" && tags.latest === version) {
  console.log(`${packageName} latest moved during a beta release`);
  process.exit(65);
}
if (tags[expectedTag] !== version) {
  console.log(`${packageName} ${expectedTag} points to ${tags[expectedTag]}, want ${version}`);
  process.exit(75);
}
console.log(`${packageName} ${expectedTag} points to ${version}`);
NODE
    )"; then
      evaluation_status=0
    else
      evaluation_status=$?
    fi

    case "$evaluation_status" in
      0) ;;
      65) fail "$evaluation" ;;
      75) all_ready=false ;;
      *) fail "npm policy evaluator failed for ${package_name} with exit ${evaluation_status}: ${evaluation}" ;;
    esac
    observations+=("$evaluation")
  done

  last_observation="$(printf '%s; ' "${observations[@]}")"
  printf 'npm readiness attempt %d: %s\n' "$attempt" "$last_observation"
  if [[ "$all_ready" == "true" ]]; then
    attempt_label="attempts"
    if ((attempt == 1)); then
      attempt_label="attempt"
    fi
    printf 'npm release policy converged after %d %s\n' "$attempt" "$attempt_label"
    exit 0
  fi
  if ((SECONDS >= deadline)); then
    fail "npm release policy did not converge within ${timeout_seconds}s after ${attempt} attempts: ${last_observation}"
  fi

  sleep "$poll_interval_seconds"
done
