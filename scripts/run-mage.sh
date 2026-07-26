#!/usr/bin/env bash

set -euo pipefail

if (($# < 1)); then
  echo "usage: scripts/run-mage.sh <mage-version> [target...]" >&2
  exit 2
fi

mage_version=$1
shift

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cache_base=${AGH_MAGE_CACHE_DIR:-${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}/agh-dev/mage}
mkdir -p "$cache_base"

repo_lock_key=$(printf '%s\0' "$repo_root" | git hash-object --stdin)
lock_dir="$cache_base/compile-$repo_lock_key.lock"
lock_owner="$lock_dir/owner"
generated_main="$repo_root/magefiles/mage_output_file.go"
temporary_dir=
temporary_binary=
lock_held=false

release_lock() {
  if [[ "$lock_held" == true ]]; then
    rm -f "$generated_main"
    rm -f "$lock_owner"
    rmdir "$lock_dir" 2>/dev/null || true
    lock_held=false
  fi
  if [[ -n "$temporary_binary" ]]; then
    rm -f "$temporary_binary"
  fi
  if [[ -n "$temporary_dir" ]]; then
    rmdir "$temporary_dir" 2>/dev/null || true
  fi
}
trap release_lock EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

acquire_lock() {
  local owner_pid=
  local ownerless_checks=0
  while ! mkdir "$lock_dir" 2>/dev/null; do
    owner_pid=
    if [[ -r "$lock_owner" ]] && read -r owner_pid < "$lock_owner" &&
      [[ "$owner_pid" =~ ^[0-9]+$ ]]; then
      ownerless_checks=0
      if ! kill -0 "$owner_pid" 2>/dev/null; then
        rm -f "$lock_owner"
        rmdir "$lock_dir" 2>/dev/null || true
        continue
      fi
    else
      ownerless_checks=$((ownerless_checks + 1))
      if ((ownerless_checks >= 20)); then
        rm -f "$lock_owner"
        rmdir "$lock_dir" 2>/dev/null || true
        ownerless_checks=0
        continue
      fi
    fi
    sleep 0.05
  done
  printf '%s\n' "$$" > "$lock_owner"
  lock_held=true
}

acquire_lock
rm -f "$generated_main"

go_cmd=$(command -v go)
source_files=()
while IFS= read -r source_file; do
  if [[ -n "$source_file" ]]; then
    source_files+=("$source_file")
  fi
done < <(
  cd "$repo_root"
  "$go_cmd" list -tags=mage -deps -f \
    '{{if .Module}}{{if .Module.Main}}{{range .GoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}{{range .EmbedFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}' \
    ./magefiles | sed '/^$/d' | LC_ALL=C sort -u
)
source_files+=("$repo_root/go.mod" "$repo_root/go.sum" "$repo_root/scripts/run-mage.sh")

go_environment=$(
  cd "$repo_root"
  "$go_cmd" env -json \
    GOOS GOARCH GOVERSION CGO_ENABLED GOAMD64 GOARM GOARM64 GOMIPS GOMIPS64 GORISCV64 GOWASM \
    GOFLAGS GOEXPERIMENT GOTOOLCHAIN GOWORK CC CXX CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS PKG_CONFIG
)
fingerprint=$(
  {
    printf 'fingerprint-version=2\n'
    printf 'mage-version=%s\n' "$mage_version"
    printf 'go-command=%s\n' "$go_cmd"
    printf 'go-environment=%s\n' "$go_environment"
    for source_file in "${source_files[@]}"; do
      printf '%s\0' "${source_file#"$repo_root"/}"
    done
    git -C "$repo_root" hash-object "${source_files[@]}"
  } | git hash-object --stdin
)

binary="$cache_base/magefile-$fingerprint"
if [[ ! -x "$binary" ]]; then
  temporary_dir="$cache_base/.compile-$fingerprint-$$"
  temporary_binary="$temporary_dir/mage"
  mkdir "$temporary_dir"
  (
    cd "$repo_root"
    "$go_cmd" run "github.com/magefile/mage@$mage_version" \
      -d "$repo_root/magefiles" \
      -gocmd "$go_cmd" \
      -multiline=false \
      -compile "$temporary_binary"
  )
  chmod 0755 "$temporary_binary"
  mv "$temporary_binary" "$binary"
fi
release_lock

cd "$repo_root"
exec "$binary" "$@"
