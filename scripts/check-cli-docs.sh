#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docs_dir="$repo_root/packages/site/content/runtime/cli-reference"
tmp_root="$(mktemp -d "${TMPDIR:-/tmp}/compozy-cli-docs-check.XXXXXX")"
candidate_dir="$tmp_root/cli-reference"

cleanup() {
  rm -rf -- "$tmp_root"
}
trap cleanup EXIT

mkdir -p "$candidate_dir"
cp "$docs_dir/index.mdx" "$docs_dir/meta.json" "$candidate_dir/"

(
  cd "$repo_root"
  go run ./cmd/compozy doc --output-dir "$candidate_dir"
  bunx oxfmt "$candidate_dir"
)

if ! diff -ru "$docs_dir" "$candidate_dir"; then
  echo "CLI reference drift detected. Run 'make cli-docs' and commit the result." >&2
  exit 1
fi

echo "CLI reference is up to date."
