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

version="${1:-}"
npm_tag="${2:-}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] ||
  fail "release version must be strict unprefixed SemVer"
[[ "$npm_tag" =~ ^[a-z][a-z0-9._-]*$ ]] || fail "npm tag is invalid"

for command in bun npm node; do
  command -v "$command" >/dev/null 2>&1 || fail "$command is required"
done

package_name="@compozy/extension-sdk"
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

bunx turbo run build --filter=@compozy/extension-sdk

package_dir="$(mktemp -d)"
cleanup_package() {
  rm -rf "$package_dir"
}
trap cleanup_package EXIT
cp sdk/typescript/package.json sdk/typescript/LICENSE "$package_dir/"
cp -R sdk/typescript/dist "$package_dir/dist"

(
  cd "$package_dir"
  npm version "$version" --no-git-tag-version --allow-same-version >/dev/null
)

publish_args=("$package_dir" --access public --tag "$npm_tag")
if [[ "$mode" == "dry-run" ]]; then
  publish_args+=(--dry-run)
fi
npm publish "${publish_args[@]}"
