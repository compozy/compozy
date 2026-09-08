#!/usr/bin/env bash
# Turbo's //#codegen-check root task lands here. Inside `make verify`, mage runs
# the full CodegenCheck first and exports COMPOZY_CODEGEN_CHECKED=1 so the turbo
# re-entry is a no-op instead of a second regenerate-and-diff pass. Standalone
# runs reuse the gate's content-keyed evidence; make codegen-check always checks.
set -euo pipefail

if [ "${COMPOZY_CODEGEN_CHECKED:-}" = "1" ]; then
  echo "codegen-check: satisfied by mage CodegenCheck in this pipeline (COMPOZY_CODEGEN_CHECKED=1)"
  exit 0
fi

exec bash scripts/gate.sh codegen
