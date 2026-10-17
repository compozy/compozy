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

# Use the checkout version, not a stale release tag, for extension compatibility.
# Development metadata keeps the version valid for the desktop handshake without
# identifying a published SDK. The commit date keeps unchanged rebuilds identical.
version_package="github.com/compozy/compozy/internal/version"
git_version=$(git describe --tags --match 'v*' --always --dirty 2>/dev/null) || git_version=
commit=$(git rev-parse --short HEAD 2>/dev/null) || commit=
[[ -n "$commit" ]] || commit=unknown
version=$(bun --print 'require("./package.json").version')
version="v${version}+dev.${commit}"
if [[ "$git_version" == *-dirty ]]; then
  version+="-dirty"
fi
build_date=$(git show -s --format=%cI HEAD 2>/dev/null) || build_date=
[[ -n "$build_date" ]] || build_date=unknown

mkdir -p "$build_dir"
# Go reuses an executable whose build ID matches the current inputs. Seed the
# next output so unchanged rebuilds can skip linking, while failed builds still
# leave the running binary untouched. Go validates source, embed, and flag changes.
if [[ -f "$binary" ]]; then
  cp -p -- "$binary" "$next_binary"
fi
go build \
  -ldflags "-X ${version_package}.Version=${version} -X ${version_package}.Commit=${commit} -X ${version_package}.BuildDate=${build_date}" \
  -o "$next_binary" ./cmd/compozy
mv -f -- "$next_binary" "$binary"
