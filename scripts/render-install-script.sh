#!/usr/bin/env bash
# Renders scripts/install.template.sh into a distributable install.sh with the
# release tag baked in as the pinned default. The published installer must keep
# an explicit tag (no client-side "latest" resolution); publishers inject it:
# goreleaser injects the release tag, and the site route injects the latest
# published release.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: render-install-script.sh <vX.Y.Z[-prerelease]> <output-path>" >&2
  exit 1
fi

tag="$1"
output="$2"
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
template="$repo_root/scripts/install.template.sh"
placeholder="__COMPOZY_PINNED_VERSION__"

if [[ ! -f "$template" ]]; then
  echo "render-install-script: template not found: $template" >&2
  exit 1
fi

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "render-install-script: release tag must be a v-prefixed semver tag, got: $tag" >&2
  exit 1
fi

if ! grep -Fq "$placeholder" "$template"; then
  echo "render-install-script: template is missing placeholder $placeholder" >&2
  exit 1
fi

rendered="${tag//\\/\\\\}"
rendered="${rendered//&/\\&}"
mkdir -p "$(dirname -- "$output")"
sed "s|$placeholder|$rendered|g" "$template" >"$output"

if grep -Fq "$placeholder" "$output"; then
  echo "render-install-script: placeholder survived rendering in $output" >&2
  exit 1
fi

sh -n "$output"
echo "render-install-script: wrote $output pinned to $tag"
