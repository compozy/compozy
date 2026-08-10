#!/usr/bin/env bash
set -euo pipefail

candidate="${1:?candidate version is required}"
live="${2:-}"

go run ./cmd/compozy-desktop-release assert-version \
  --candidate "${candidate}" \
  --live "${live}"

