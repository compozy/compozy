#!/usr/bin/env bash
set -euo pipefail

scope="${1:-release-candidate}"
base_dir="${2:-"$HOME/dev/qa-labs"}"

slug="$(printf '%s' "$scope" \
  | tr '[:upper:]' '[:lower:]' \
  | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g')"

if [[ -z "$slug" ]]; then
  slug="release-candidate"
fi

workspace_path="$base_dir/agh-$slug-lab"
qa_output_path="$workspace_path/qa-artifacts"

mkdir -p \
  "$workspace_path/company" \
  "$workspace_path/product" \
  "$workspace_path/marketing" \
  "$workspace_path/finance" \
  "$workspace_path/ops" \
  "$workspace_path/reviews" \
  "$workspace_path/.agh" \
  "$qa_output_path/qa/test-plans" \
  "$qa_output_path/qa/test-cases" \
  "$qa_output_path/qa/issues" \
  "$qa_output_path/qa/screenshots"

cat <<EOF
SCENARIO_SLUG=$slug
WORKSPACE_PATH=$workspace_path
QA_OUTPUT_PATH=$qa_output_path
EOF
