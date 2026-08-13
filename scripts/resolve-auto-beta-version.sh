#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

for command in git node; do
  command -v "$command" >/dev/null 2>&1 || fail "${command} is required"
done

base="$(node --input-type=module -e '
  import { readFileSync } from "node:fs";
  const manifest = JSON.parse(readFileSync("package.json", "utf8"));
  if (typeof manifest.version !== "string") {
    throw new Error("package.json version must be a string");
  }
  process.stdout.write(manifest.version);
')"
if [[ ! "$base" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  fail "release PR package version '${base}' is not plain SemVer; cannot derive the beta version"
fi

last=0
while IFS= read -r tag; do
  suffix="${tag#v"${base}"-beta.}"
  if [[ "$suffix" =~ ^[0-9]+$ ]] && ((suffix > last)); then
    last="$suffix"
  fi
done < <(git tag --list "v${base}-beta.*")

printf '%s-beta.%d\n' "$base" "$((last + 1))"
