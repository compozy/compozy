#!/usr/bin/env bash
set -euo pipefail

package_spec=${1:?usage: smoke-public-cli-package.sh <package-spec> <version>}
release_version=${2:?release version is required}
smoke_parent=${RUNNER_TEMP:-${TMPDIR:-/tmp}}
smoke_root=$(mktemp -d "${smoke_parent}/compozy-cli-public-smoke.XXXXXX")
prefix="${smoke_root}/prefix"

npm install \
  --prefix "${prefix}" \
  --cache "${smoke_root}/npm-cache" \
  --no-audit \
  --no-fund \
  "${package_spec}"

installed_version=$(node -e \
  'process.stdout.write(require(process.argv[1]).version)' \
  "${prefix}/node_modules/@compozy/cli/package.json")
[[ "${installed_version}" == "${release_version}" ]] || {
  echo "CLI package version ${installed_version} does not match ${release_version}" >&2
  exit 1
}

version_output=$("${prefix}/node_modules/.bin/compozy" version)
if [[ "${version_output}" != *"${release_version}"* ]]; then
  echo "CLI binary version output does not contain ${release_version}: ${version_output}" >&2
  exit 1
fi
printf 'public CLI package smoke: PASS (%s)\n' "${release_version}"
