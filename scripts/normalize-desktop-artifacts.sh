#!/usr/bin/env bash
set -euo pipefail

platform="${1:?platform is required}"
release_version="${2:?release version is required}"
bundle_root="${3:?Tauri bundle root is required}"
output_dir="${4:?output directory is required}"

mkdir -p "${output_dir}"

copy_one() {
  local pattern="$1"
  local destination="$2"
  local matches=()
  while IFS= read -r match; do
    matches+=("${match}")
  done < <(find "${bundle_root}" -type f -path "${pattern}" -print)
  if [[ "${#matches[@]}" != 1 ]]; then
    echo "desktop release: ${pattern} matched ${#matches[@]} artifacts, want exactly one" >&2
    exit 1
  fi
  cp "${matches[0]}" "${output_dir}/${destination}"
}

case "${platform}" in
  macos)
    copy_one '*/bundle/macos/*.app.tar.gz' "CompozyOS_${release_version}_universal.app.tar.gz"
    copy_one '*/bundle/macos/*.app.tar.gz.sig' "CompozyOS_${release_version}_universal.app.tar.gz.sig"
    copy_one '*/bundle/dmg/*.dmg' "CompozyOS_${release_version}_universal.dmg"
    ;;
  linux)
    copy_one '*/bundle/appimage/*.AppImage' "CompozyOS_${release_version}_amd64.AppImage"
    copy_one '*/bundle/appimage/*.AppImage.sig' "CompozyOS_${release_version}_amd64.AppImage.sig"
    copy_one '*/bundle/deb/*.deb' "CompozyOS_${release_version}_amd64.deb"
    ;;
  windows)
    copy_one '*/bundle/nsis/*-setup.exe' "CompozyOS_${release_version}_x64-setup.exe"
    copy_one '*/bundle/nsis/*-setup.exe.sig' "CompozyOS_${release_version}_x64-setup.exe.sig"
    ;;
  *)
    echo "desktop release: unsupported normalization platform ${platform}" >&2
    exit 2
    ;;
esac

