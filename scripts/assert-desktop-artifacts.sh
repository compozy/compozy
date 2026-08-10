#!/usr/bin/env bash
set -euo pipefail

artifact_dir="${1:?normalized desktop artifact directory is required}"
release_version="${2:?release version is required}"

go run ./cmd/compozy-desktop-release assert-inventory \
  --dir "${artifact_dir}" \
  --version "${release_version}"

