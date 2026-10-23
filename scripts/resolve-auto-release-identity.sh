#!/usr/bin/env bash
set -euo pipefail

# Resolves the version and channel an automatic release publishes from the merged release PR.
# Stable releases publish the release PR version; beta releases derive the next beta from it.

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

readonly channel="${1:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

case "$channel" in
  stable)
    command -v node >/dev/null 2>&1 || fail "node is required"
    version="$(node --input-type=module -e '
      import { readFileSync } from "node:fs";
      const manifest = JSON.parse(readFileSync("package.json", "utf8"));
      if (typeof manifest.version !== "string") {
        throw new Error("package.json version must be a string");
      }
      process.stdout.write(manifest.version);
    ')"
    [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
      fail "release PR package version '${version}' is not plain SemVer; cannot publish a stable release"
    ;;
  beta)
    version="$(bash "${SCRIPT_DIR}/resolve-auto-beta-version.sh")"
    ;;
  *)
    fail "automatic release channel must be stable or beta, got '${channel}'"
    ;;
esac

printf 'release_version=%s\nrelease_channel=%s\n' "$version" "$channel"
