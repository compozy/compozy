#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

build_dir=${COMPOZY_AIR_BUILD_DIR:-"$repo_root/.tmp/air"}
binary="$build_dir/compozy"
next_binary="$build_dir/compozy.next"

cleanup() {
  local exit_status=$?
  trap - EXIT

  if [[ -e "$next_binary" ]] && ! rm -f -- "$next_binary"; then
    echo "dev: failed to remove incomplete daemon binary $next_binary" >&2
    if ((exit_status == 0)); then
      exit_status=1
    fi
  fi

  exit "$exit_status"
}

trap cleanup EXIT

# Version and commit mirror buildLDFlags in magefiles/build.go: the desktop
# shell's runtime handshake rejects daemons whose reported version does not
# parse as semver, so a bare "dev" build can never attach. BuildDate uses the
# commit date (not now) so rebuilds without code changes keep identical bytes
# and scripts/dev-daemon.sh can skip the daemon restart.
version_package="github.com/compozy/compozy/internal/version"
version=$(git describe --tags --match 'v*' --always --dirty 2>/dev/null) || version=
[[ -n "$version" ]] || version=dev
commit=$(git rev-parse --short HEAD 2>/dev/null) || commit=
[[ -n "$commit" ]] || commit=unknown
build_date=$(git show -s --format=%cI HEAD 2>/dev/null) || build_date=
[[ -n "$build_date" ]] || build_date=unknown

mkdir -p "$build_dir"
go build \
  -ldflags "-X ${version_package}.Version=${version} -X ${version_package}.Commit=${commit} -X ${version_package}.BuildDate=${build_date}" \
  -o "$next_binary" ./cmd/compozy
mv -f -- "$next_binary" "$binary"
