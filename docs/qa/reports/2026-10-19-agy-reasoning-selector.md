# Agy reasoning selector regression — 2026-10-19

Verdict: PASS for the reasoning-selector slice of `RT-agy-acp-provider`.

The built-in Agy provider previously defaulted to `reasoning_apply: none`, discarding its advertised
`thinking_level` values from the catalog's selectable efforts. It now uses the existing `acp_option`
strategy. Explicit `none` overrides still win; model IDs, credentials, and persisted shapes are unchanged.

## Evidence

- The daemon catalog regression reproduced the failure before the fix and passes afterward with
  exact bare model IDs, model-specific Low/Medium/High, the High default, explicit opt-out, and context
  isolation. Configuration and model-catalog packages and the daemon catalog wiring suite pass with
  `-race`; focused Agy settings/API/CLI suites pass with `-race`.
- `golangci-lint run ./internal/config ./internal/daemon` reports zero issues. The changed browser test
  passes formatting and warning-denying lint checks.
- The existing daemon-served AGY browser case passes: choose Low in the session model picker, submit
  a prompt containing `reasoning_effort: low`, and observe the same effective runtime and exact model.
  This test uses a deterministic ACP peer, not a Google account.
- A rebuilt binary in an isolated daemon home inspected the real AGY server. Both advertised models
  publish Low/Medium/High, default High, and `reasoning_apply: acp_option`; CLI and HTTP projections match.
  No account-backed prompt was sent.
- Web build and typecheck pass through Turborepo. Build warnings about CSS `::highlight`, chunk sizes,
  and a static/dynamic import are outside this change. Full branch `make gate` and PR CI were not run.

Scratch evidence and the bootstrap manifest are under
`/tmp/opencode/compozy-agy-reasoning-selector-20261019-115939-561633-lab/qa-artifacts/qa/`.
The strict evidence audit passes; that directory's `teardown.json` records `"clean": true`.
The historical authentication/first-prompt walkthrough remains in `2026-10-16-agy-acp.md`; it was not
repeated for this targeted regression.
