#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

for command in git node npm; do
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

for package_name in @compozy/cli @compozy/extension-sdk; do
  registry_versions="$(
    npm view "${package_name}" versions --json --registry=https://registry.npmjs.org
  )" || fail "could not read published ${package_name} versions"
  published_versions="$(
    node -e '
      const value = JSON.parse(process.argv[1]);
      const versions = Array.isArray(value) ? value : [value];
      for (const version of versions) {
        if (typeof version !== "string") {
          throw new Error("npm returned a non-string package version");
        }
        process.stdout.write(`${version}\n`);
      }
    ' "$registry_versions"
  )" || fail "npm returned invalid ${package_name} version metadata"
  while IFS= read -r version; do
    suffix="${version#"${base}"-beta.}"
    if [[ "$suffix" =~ ^[0-9]+$ ]] && ((suffix > last)); then
      last="$suffix"
    fi
  done <<<"$published_versions"
done

printf '%s-beta.%d\n' "$base" "$((last + 1))"
