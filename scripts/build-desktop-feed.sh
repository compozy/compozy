#!/usr/bin/env bash
set -euo pipefail

desktop_dir="${1:?normalized desktop artifact directory is required}"
runtime_dir="${2:?runtime artifact directory is required}"
release_version="${3:?release version is required}"
published_at="${4:?RFC3339 publication time is required}"
output_dir="${5:?feed output directory is required}"

go run ./cmd/compozy-desktop-release build-feed \
  --version "${release_version}" \
  --published-at "${published_at}" \
  --desktop-dir "${desktop_dir}" \
  --runtime-dir "${runtime_dir}" \
  --output-dir "${output_dir}"
