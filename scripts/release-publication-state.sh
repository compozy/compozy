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

write_decision() {
  local state="$1"
  local already_published="$2"
  local release_id="$3"

  {
    printf 'publication_state=%s\n' "$state"
    printf 'already_published=%s\n' "$already_published"
    printf 'release_id=%s\n' "$release_id"
  } >>"$GITHUB_OUTPUT"
}

asset_by_name() {
  local release_json="$1"
  local asset_name="$2"
  local matches
  matches="$(jq -c --arg name "$asset_name" '[.assets[] | select(.name == $name)]' <<<"$release_json")"

  local count
  count="$(jq -r 'length' <<<"$matches")"
  [[ "$count" == "1" ]] || fail "Published recovery requires exactly one ${asset_name} asset, found ${count}"

  local asset
  asset="$(jq -c '.[0]' <<<"$matches")"
  [[ "$(jq -r '.state' <<<"$asset")" == "uploaded" ]] ||
    fail "Published recovery asset ${asset_name} is not uploaded"
  (( "$(jq -r '.size' <<<"$asset")" > 0 )) ||
    fail "Published recovery asset ${asset_name} is empty"

  printf '%s\n' "$asset"
}

published_asset_name() {
  local checksum_name="$1"
  case "$checksum_name" in
    *.deb | *.deb.sbom.json | *.rpm | *.rpm.sbom.json)
      printf '%s\n' "${checksum_name//\~/.}"
      ;;
    *)
      printf '%s\n' "$checksum_name"
      ;;
  esac
}

for command in gh jq npm; do
  command -v "$command" >/dev/null 2>&1 || fail "${command} is required"
done

for name in GITHUB_OUTPUT GITHUB_REPOSITORY NPM_TOKEN RELEASE_COMMIT RELEASE_TAG RELEASE_VERSION RUNNER_TEMP; do
  require_env "$name"
done

npm whoami --registry=https://registry.npmjs.org >/dev/null

release_pages="$(
  gh api --paginate --slurp "repos/${GITHUB_REPOSITORY}/releases?per_page=100"
)" || fail "Could not list GitHub Releases"
matching_releases="$(
  jq -c --arg tag "$RELEASE_TAG" '[.[][] | select(.tag_name == $tag)]' <<<"$release_pages"
)" || fail "Could not resolve the GitHub Release inventory"
matching_count="$(jq -r 'length' <<<"$matching_releases")"

work_dir="$(mktemp -d "${RUNNER_TEMP}/release-publication-state.XXXXXX")"
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

cli_state="$(package_state "@compozy/cli" "cli")"
extension_sdk_state="$(package_state "@compozy/extension-sdk" "extension-sdk")"

if [[ "$cli_state" == "published" ]]; then
  [[ "$matching_count" == "1" ]] ||
    fail "@compozy/cli is published but expected exactly one matching GitHub Release, found ${matching_count}"

  release_json="$(jq -c '.[0]' <<<"$matching_releases")"
  release_id="$(jq -r '.id' <<<"$release_json")"
  actual_release_commit="$(jq -r '.target_commitish' <<<"$release_json")"
  [[ "$actual_release_commit" == "$RELEASE_COMMIT" ]] ||
    fail "GitHub Release target is ${actual_release_commit}, want ${RELEASE_COMMIT}"
  [[ "$(jq -r '.draft' <<<"$release_json")" == "true" ]] ||
    fail "npm is published but the tagless GitHub Release is not a draft"

  checksums_asset="$(asset_by_name "$release_json" "checksums.txt")"
  asset_by_name "$release_json" "checksums.txt.sigstore.json" >/dev/null
  asset_by_name "$release_json" "install.sh" >/dev/null

  checksums_file="${work_dir}/checksums.txt"
  checksums_asset_id="$(jq -r '.id' <<<"$checksums_asset")"
  gh api \
    -H "Accept: application/octet-stream" \
    "repos/${GITHUB_REPOSITORY}/releases/assets/${checksums_asset_id}" >"$checksums_file" ||
    fail "Could not download checksums.txt from GitHub Release ${release_id}"
  [[ -s "$checksums_file" ]] ||
    fail "Published recovery checksums.txt is empty"

  checksum_count=0
  while read -r digest artifact extra; do
    [[ -n "$digest" && -n "$artifact" && -z "${extra:-}" ]] ||
      fail "Published recovery checksums.txt contains a malformed line"
    [[ "$digest" =~ ^[0-9a-f]{64}$ ]] ||
      fail "Published recovery checksum for ${artifact} is not SHA-256"

    published_artifact="$(published_asset_name "$artifact")"
    asset_by_name "$release_json" "$published_artifact" >/dev/null
    ((checksum_count += 1))
  done <"$checksums_file"
  ((checksum_count > 0)) ||
    fail "Published recovery checksums.txt contains no artifacts"

  if [[ "$extension_sdk_state" == "published" ]]; then
    write_decision "orphaned_complete" "true" "$release_id"
    printf 'Publication state: orphaned_complete (release %s, %d checksum-backed assets)\n' \
      "$release_id" "$checksum_count"
  else
    write_decision "orphaned_cli_only" "true" "$release_id"
    printf 'Publication state: orphaned_cli_only (release %s; extension SDK publish will resume)\n' \
      "$release_id"
  fi
  exit 0
fi

[[ "$extension_sdk_state" == "missing" ]] ||
  fail "@compozy/extension-sdk is published while @compozy/cli is missing"
[[ "$matching_count" == "0" ]] ||
  fail "GitHub Release exists but npm ${RELEASE_VERSION} is missing"

write_decision "fresh" "false" ""
printf 'Publication state: fresh (npm packages at %s are unpublished)\n' "$RELEASE_VERSION"
