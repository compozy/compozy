#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

mode="publish"
if [[ "${1:-}" == "--dry-run" ]]; then
  mode="dry-run"
  shift
fi

source_dir="${1:-}"
version="${2:-}"
npm_tag="${3:-}"
[[ $# == 3 ]] || fail "usage: publish-sdk-package.sh [--dry-run] <package-dir> <version> <npm-tag>"
strict_semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*)|([0-9]*[A-Za-z-][0-9A-Za-z-]*))(\.((0|[1-9][0-9]*)|([0-9]*[A-Za-z-][0-9A-Za-z-]*)))*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
[[ "$version" =~ $strict_semver ]] ||
  fail "release version must be strict unprefixed SemVer"
[[ "$npm_tag" =~ ^[a-z][a-z0-9._-]*$ ]] || fail "npm tag is invalid"

for command in bun bunx npm node; do
  command -v "$command" >/dev/null 2>&1 || fail "$command is required"
done

[[ -f "$source_dir/package.json" ]] || fail "SDK package.json is missing: ${source_dir}"
package_name="$(node -e '
  const fs = require("node:fs");
  const pkg = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  if (typeof pkg.name !== "string" || !/^@compozy\/[a-z0-9._-]+$/.test(pkg.name) || pkg.private !== false) {
    throw new Error("SDK package must be a public @compozy package");
  }
  process.stdout.write(pkg.name);
' "$source_dir/package.json")"
if [[ "$mode" == "publish" ]]; then
  npm_output="$(mktemp)"
  npm_error="$(mktemp)"
  cleanup_lookup() {
    rm -f "$npm_output" "$npm_error"
  }
  trap cleanup_lookup EXIT
  if npm view "${package_name}@${version}" version \
    --registry=https://registry.npmjs.org >"$npm_output" 2>"$npm_error"; then
    published_version="$(tr -d '[:space:]' <"$npm_output")"
    [[ "$published_version" == "$version" ]] ||
      fail "npm resolved ${published_version}, want ${version}"
    printf '%s@%s is already published\n' "$package_name" "$version"
    exit 0
  fi
  if ! grep -Eq 'E404|404 Not Found' "$npm_error" "$npm_output"; then
    cat "$npm_output" >&2
    cat "$npm_error" >&2
    fail "could not determine whether ${package_name}@${version} is published"
  fi
  cleanup_lookup
  trap - EXIT
fi

bunx turbo run build --filter="$package_name"

package_dir="$(mktemp -d)"
cleanup_package() {
  rm -rf "$package_dir"
}
trap cleanup_package EXIT
cp "$source_dir/package.json" "$source_dir/LICENSE" "$package_dir/"
if [[ -f "$source_dir/README.md" ]]; then
  cp "$source_dir/README.md" "$package_dir/"
fi
cp -R "$source_dir/dist" "$package_dir/dist"

node - "$source_dir" "$package_dir" "$version" <<'NODE'
const fs = require("node:fs");
const path = require("node:path");
const [source, stage, version] = process.argv.slice(2);
const manifestPath = path.join(stage, "package.json");
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
manifest.version = version;
fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");

if (fs.existsSync(path.join(source, "src/version.ts"))) {
  for (const entry of ["esm/version.js", "cjs/version.js", "types/version.d.ts"]) {
    const file = path.join(stage, "dist", entry);
    const contents = fs.readFileSync(file, "utf8");
    if (!contents.includes('"0.0.0-dev"')) {
      throw new Error(`${entry} must contain the development version before publication`);
    }
    fs.writeFileSync(file, contents.replaceAll('"0.0.0-dev"', JSON.stringify(version)));
  }
}
NODE

publish_args=("$package_dir" --access public --tag "$npm_tag" --provenance)
if [[ "$mode" == "dry-run" ]]; then
  publish_args+=(--dry-run)
fi
npm publish "${publish_args[@]}"
