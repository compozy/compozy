#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf '::error::%s\n' "$*" >&2
  exit 1
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || fail "Missing release asset: ${path}"
}

require_nonempty_file() {
  local path="$1"
  local message="$2"
  [[ -s "$path" ]] || fail "$message"
}

require_text() {
  local path="$1"
  local text="$2"
  local message="$3"
  grep -Fq -- "$text" "$path" || fail "$message"
}

for command in git go grep sh; do
  command -v "$command" >/dev/null 2>&1 || fail "${command} is required"
done

repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" ||
  fail "Release preflight must run inside a Git worktree"
cd "$repo_root"

require_nonempty_file \
  "RELEASE_BODY.md" \
  "Missing or empty RELEASE_BODY.md; release-pr must generate the current release body"
require_nonempty_file \
  "RELEASE_NOTES.md" \
  "Missing or empty RELEASE_NOTES.md; release-pr must generate historical release notes"
require_file ".goreleaser.release-header.md.tmpl"
require_file ".goreleaser.release-footer.md.tmpl"

readonly installer_path="packages/site/public/install.sh"
require_nonempty_file \
  "$installer_path" \
  "Missing or empty curl installer: ${installer_path}"
sh -n "$installer_path"
require_text \
  "$installer_path" \
  "checksums.txt.sigstore.json" \
  "install.sh must verify checksums.txt with checksums.txt.sigstore.json"
require_text \
  "$installer_path" \
  'COSIGN_VERSION="v2.2.4"' \
  "install.sh must bootstrap a pinned cosign verifier when cosign is absent"
require_text \
  "$installer_path" \
  "refs/heads/main" \
  "install.sh must verify checksums against the production release workflow identity"

go run github.com/magefile/mage@v1.17.2 releaseInstallCheck

require_text \
  ".goreleaser.yml" \
  "packages/site/public/install.sh" \
  ".goreleaser.yml must upload packages/site/public/install.sh as a release extra file"
require_text \
  ".goreleaser.yml" \
  "--bundle=\${signature}" \
  ".goreleaser.yml must sign checksums with a Sigstore bundle artifact"
require_text \
  ".goreleaser.yml" \
  'name: "@compozy/cli"' \
  ".goreleaser.yml must publish the @compozy/cli npm package"

worktree_state="$(git status --porcelain --untracked-files=normal)"
if [[ -n "$worktree_state" ]]; then
  printf '::error::Release worktree must be clean before publication\n' >&2
  printf '%s\n' "$worktree_state" >&2
  exit 1
fi

printf 'release preflight: PASS\n'
