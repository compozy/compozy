#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

require_env() {
  local name="$1"
  [[ -n "${!name:-}" ]] || fail "${name} is required"
}

package_state() {
  local package_name="$1"
  local file_slug="$2"
  local npm_view_output="${work_dir}/${file_slug}.out"
  local npm_view_error="${work_dir}/${file_slug}.err"

  if npm view "${package_name}@${RELEASE_VERSION}" version \
    --registry=https://registry.npmjs.org >"$npm_view_output" 2>"$npm_view_error"; then
    local published_version
    published_version="$(tr -d '[:space:]' <"$npm_view_output")"
    [[ "$published_version" == "$RELEASE_VERSION" ]] ||
      fail "npm resolved ${package_name}@${published_version}, want ${RELEASE_VERSION}"
    printf 'published\n'
    return
  fi
  if grep -Eq 'E404|404 Not Found' "$npm_view_error" "$npm_view_output"; then
    printf 'missing\n'
    return
  fi
  cat "$npm_view_output" >&2
  cat "$npm_view_error" >&2
  fail "Could not determine whether ${package_name}@${RELEASE_VERSION} is published"
}

write_decision() {
  {
    printf 'publication_state=%s\n' "$1"
    printf 'release_state=%s\n' "$2"
    printf 'cli_state=%s\n' "$3"
    printf 'extension_sdk_state=%s\n' "$4"
    printf 'stage_release=%s\n' "$5"
    printf 'desktop_required=%s\n' "$6"
    printf 'release_id=%s\n' "$7"
  } >>"$GITHUB_OUTPUT"
}

asset_by_name() {
  local release_json="$1"
  local asset_name="$2"
  local matches
  matches="$(jq -c --arg name "$asset_name" '[.assets[] | select(.name == $name)]' <<<"$release_json")"
  local count
  count="$(jq -r 'length' <<<"$matches")"
  [[ "$count" == "1" ]] || fail "Expected exactly one ${asset_name} asset, found ${count}"
  local asset
  asset="$(jq -c '.[0]' <<<"$matches")"
  [[ "$(jq -r '.state' <<<"$asset")" == "uploaded" ]] ||
    fail "Release asset ${asset_name} is not uploaded"
  (("$(jq -r '.size' <<<"$asset")" > 0)) || fail "Release asset ${asset_name} is empty"
  printf '%s\n' "$asset"
}

published_asset_name() {
  case "$1" in
    *.deb | *.deb.sbom.json | *.rpm | *.rpm.sbom.json) printf '%s\n' "${1//\~/.}" ;;
    *) printf '%s\n' "$1" ;;
  esac
}

verify_release_assets() {
  local release_json="$1"
  local checksums_asset
  checksums_asset="$(asset_by_name "$release_json" "checksums.txt")"
  asset_by_name "$release_json" "checksums.txt.sigstore.json" >/dev/null
  asset_by_name "$release_json" "install.sh" >/dev/null
  local checksums_file="${work_dir}/checksums.txt"
  local checksums_asset_id
  checksums_asset_id="$(jq -r '.id' <<<"$checksums_asset")"
  gh api -H "Accept: application/octet-stream" \
    "repos/${GITHUB_REPOSITORY}/releases/assets/${checksums_asset_id}" >"$checksums_file" ||
    fail "Could not download checksums.txt from GitHub Release"
  [[ -s "$checksums_file" ]] || fail "Release checksums.txt is empty"
  local checksum_count=0 digest artifact extra published_artifact
  while read -r digest artifact extra; do
    [[ -n "$digest" && -n "$artifact" && -z "${extra:-}" ]] ||
      fail "Release checksums.txt contains a malformed line"
    [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || fail "Checksum for ${artifact} is not SHA-256"
    published_artifact="$(published_asset_name "$artifact")"
    asset_by_name "$release_json" "$published_artifact" >/dev/null
    ((checksum_count += 1))
  done <"$checksums_file"
  ((checksum_count > 0)) || fail "Release checksums.txt contains no artifacts"
}

for command in gh jq npm; do
  command -v "$command" >/dev/null 2>&1 || fail "${command} is required"
done
for name in GITHUB_OUTPUT GITHUB_REPOSITORY NPM_TOKEN RELEASE_COMMIT RELEASE_TAG RELEASE_VERSION RUNNER_TEMP; do
  require_env "$name"
done

npm whoami --registry=https://registry.npmjs.org >/dev/null
work_dir="$(mktemp -d "${RUNNER_TEMP}/release-publication-state.XXXXXX")"
cli_state="$(package_state "@compozy/cli" "cli")"
extension_sdk_state="$(package_state "@compozy/extension-sdk" "extension-sdk")"
if [[ "$extension_sdk_state" == "published" && "$cli_state" != "published" ]]; then
  fail "@compozy/extension-sdk is published while @compozy/cli is missing"
fi

release_pages="$(
  gh api --paginate --slurp "repos/${GITHUB_REPOSITORY}/releases?per_page=100"
)" || fail "Could not list GitHub Releases"
matching_releases="$(
  jq -c --arg tag "$RELEASE_TAG" '[.[][] | select(.tag_name == $tag)]' <<<"$release_pages"
)" || fail "Could not resolve the GitHub Release inventory"
matching_count="$(jq -r 'length' <<<"$matching_releases")"
((matching_count <= 1)) ||
  fail "Expected at most one matching GitHub Release, found ${matching_count}"

release_state=missing
release_id=
if [[ "$matching_count" == "1" ]]; then
  release_json="$(jq -c '.[0]' <<<"$matching_releases")"
  release_id="$(jq -r '.id' <<<"$release_json")"
  actual_release_commit="$(jq -r '.target_commitish' <<<"$release_json")"
  [[ "$actual_release_commit" == "$RELEASE_COMMIT" ]] ||
    fail "GitHub Release target is ${actual_release_commit}, want ${RELEASE_COMMIT}"
  case "$(jq -r '.draft' <<<"$release_json")" in
    true) release_state=draft ;;
    false) release_state=published ;;
    *) fail "GitHub Release returned an invalid draft state" ;;
  esac
  verify_release_assets "$release_json"
fi

stage_release=false
desktop_required=false
if [[ "$release_state" == "missing" ]]; then
  stage_release=true
fi
if [[ "$cli_state" == "missing" && "$release_state" != "published" ]]; then
  desktop_required=true
fi

state="${release_state}_${cli_state}_${extension_sdk_state}"
write_decision \
  "$state" "$release_state" "$cli_state" "$extension_sdk_state" \
  "$stage_release" "$desktop_required" "$release_id"
printf 'Publication state: %s (stage release: %s; desktop required: %s)\n' \
  "$state" "$stage_release" "$desktop_required"
