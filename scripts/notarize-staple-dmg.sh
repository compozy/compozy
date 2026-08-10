#!/usr/bin/env bash
set -euo pipefail

app_path="${1:?app bundle path is required}"
dmg_path="${2:?DMG path is required}"
release_version="${3:?release version is required}"

for name in APPLE_API_ISSUER APPLE_API_KEY APPLE_API_KEY_PATH; do
  if [[ -z "${!name:-}" ]]; then
    echo "desktop release: ${name} is required for ASC API notarization" >&2
    exit 1
  fi
done
if [[ ! -d "${app_path}" || ! -s "${dmg_path}" ]]; then
  echo "desktop release: app bundle and DMG must exist before notarization" >&2
  exit 1
fi

bundle_version="$(/usr/libexec/PlistBuddy -c 'Print:CFBundleShortVersionString' "${app_path}/Contents/Info.plist")"
if [[ "${bundle_version}" != "${release_version}" ]]; then
  echo "desktop release: CFBundleShortVersionString ${bundle_version} does not match ${release_version}" >&2
  exit 1
fi

mount_point="$(mktemp -d)"
mounted=false
mounted_apps="$(mktemp)"
cleanup() {
  status=$?
  trap - EXIT
  if [[ "${mounted}" == true ]]; then
    if ! hdiutil detach "${mount_point}" >/dev/null; then
      echo "desktop release: failed to detach DMG mount ${mount_point}" >&2
      status=1
    fi
    mounted=false
  fi
  if ! rm -f "${mounted_apps}"; then
    echo "desktop release: failed to remove mounted app inventory" >&2
    status=1
  fi
  if [[ -d "${mount_point}" ]] && ! rmdir "${mount_point}"; then
    echo "desktop release: failed to remove DMG mount directory ${mount_point}" >&2
    status=1
  fi
  exit "${status}"
}
trap cleanup EXIT
hdiutil attach -readonly -nobrowse -mountpoint "${mount_point}" "${dmg_path}" >/dev/null
mounted=true
if ! find "${mount_point}" -type d -name '*.app' -prune -print0 >"${mounted_apps}"; then
  echo "desktop release: failed to inspect the app bundle inside the DMG" >&2
  exit 1
fi
apps=()
while IFS= read -r -d '' mounted_app; do
  apps+=("${mounted_app}")
done <"${mounted_apps}"
if [[ "${#apps[@]}" != 1 ]]; then
  echo "desktop release: DMG contains ${#apps[@]} app bundles, want exactly one" >&2
  exit 1
fi
dmg_bundle_version="$(/usr/libexec/PlistBuddy -c 'Print:CFBundleShortVersionString' "${apps[0]}/Contents/Info.plist")"
if [[ "${dmg_bundle_version}" != "${release_version}" ]]; then
  echo "desktop release: DMG CFBundleShortVersionString ${dmg_bundle_version} does not match ${release_version}" >&2
  exit 1
fi
xcrun notarytool submit "${dmg_path}" \
  --key "${APPLE_API_KEY_PATH}" \
  --key-id "${APPLE_API_KEY}" \
  --issuer "${APPLE_API_ISSUER}" \
  --wait
xcrun stapler staple "${dmg_path}"
xcrun stapler validate "${dmg_path}"
