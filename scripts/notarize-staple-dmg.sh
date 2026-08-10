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

xcrun notarytool submit "${dmg_path}" \
  --key "${APPLE_API_KEY_PATH}" \
  --key-id "${APPLE_API_KEY}" \
  --issuer "${APPLE_API_ISSUER}" \
  --wait
xcrun stapler staple "${dmg_path}"
xcrun stapler validate "${app_path}"
xcrun stapler validate "${dmg_path}"

bundle_version="$(/usr/libexec/PlistBuddy -c 'Print:CFBundleShortVersionString' "${app_path}/Contents/Info.plist")"
if [[ "${bundle_version}" != "${release_version}" ]]; then
  echo "desktop release: CFBundleShortVersionString ${bundle_version} does not match ${release_version}" >&2
  exit 1
fi

