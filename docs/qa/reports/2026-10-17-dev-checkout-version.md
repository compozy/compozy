# Development checkout version — 2026-10-17

## Outcome

PASS for the reported daemon-startup failure. Air now uses the checkout's
`package.json` version, with development commit metadata and the existing dirty
marker. It no longer reports `v0.1.1` from an older reachable release tag when the
checkout declares `0.3.0`. Extension compatibility checks remain enabled.

## Evidence

- Regression: the existing `scripts/devreadiness` build fixture with a `v0.1.1`
  tag and package version `0.3.0` failed before the fix, reporting `v0.1.1`.
- `go test -race ./scripts/devreadiness -count=1` passed after the fix.
- The real binary built by `scripts/build-air.sh` reported
  `v0.3.0+dev.706daa74c-dirty` and started through `scripts/dev-daemon.sh` with
  isolated home, HTTP port, Unix socket, and development ownership state.
- CLI `status -o json` and HTTP `GET /api/status` both returned runtime status.
- The lab teardown reported `TEARDOWN_ALL_CLEAN=true`; unrelated homes were not
  modified.

Scratch receipts:
`/tmp/opencode/compozy-dev-checkout-version-20261017-202739-706944-lab/qa-artifacts/qa/`
contains `bootstrap-manifest.json`, `daemon-start.log`, `cli-status.json`,
`api-status.json`, and `teardown.json`.

## Verification limits

BLOCKED: the scoped gate (`GATE_BASE=HEAD make gate`) passed script integration
and Mage tests, then failed in Go lint because the installed analyzer supports
export-data version 4 while this Go toolchain emits version 5. Receipt:
`.cache/gate/logs/go-lint-1792268884-275461.log`.

No linter suppression or toolchain/dependency change was made. The complete
`make dev` command, Vite/browser journey, release builds, and unrelated branch
changes were not revalidated. The focused process suite covers supervisor
readiness ordering; the real-daemon probe covers the reported startup boundary.
