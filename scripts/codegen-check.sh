#!/usr/bin/env bash
# Turbo's //#codegen-check root task lands here. Inside `make verify`, mage runs
# the full CodegenCheck first and exports COMPOZY_CODEGEN_CHECKED=1 so the turbo
# re-entry is a no-op instead of a second regenerate-and-diff pass. Standalone
# turbo runs (no env) still get the full gate.
set -euo pipefail

if [ "${COMPOZY_CODEGEN_CHECKED:-}" = "1" ]; then
  echo "codegen-check: satisfied by mage CodegenCheck in this pipeline (COMPOZY_CODEGEN_CHECKED=1)"
  exit 0
fi

exec make codegen-check
