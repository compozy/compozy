# BUG-20261002-overview-zero-window: An explicit zero-day overview silently becomes thirty days

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-daemon-schema, validate overview filters across public surfaces
- **Scenarios:** RT-observe-overview-cli
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md
- **Origin:** Isolated primary QA lab

## Summary

The CLI accepts an explicitly invalid usage window of zero, exits successfully, and returns a
30-day overview. HTTP and UDS reject the same requested value with 422. An automated consumer can
therefore receive a different reporting period without an error.

## Reproduction

1. Run `compozy observe overview --usage-window=0 -o json`.
2. Observe exit 0 and `usage.window_days: 30`.
3. Read `/api/observe/overview?usage_window=0` over HTTP and UDS.
4. Both return 422 with `usage_window must be 7, 30, or 90`.

**Expected:** Explicit zero is rejected; omitting the option retains the default 30-day window.
**Actual:** The CLI silently substitutes its default for an explicit zero.

## Evidence

`docs/qa/evidence/2026-10-02-untested/overview-invalid-cli-0.json`,
`overview-invalid-zero-confirm.json`, `overview-invalid-http-0.json`, and
`overview-invalid-uds-0.json`. Explicit 8, 91, -1, and nonnumeric values are refused.

## Fix

- **Root cause:** CLI validation used the numeric value to detect omission; zero is also the
  internal omitted-option sentinel. The boundary now checks whether Cobra received the flag,
  preserving the omitted default while validating every explicit value.
- **Fix commit:** `46d8b2f07`.
- **Regression test:** Existing `internal/cli/observe_test.go` owns explicit option validation.

## Verification

The initial walk and a separate zero-value command reproduced the divergence. The rebuilt CLI now refuses explicit zero with nonzero exit and the canonical validation message.
Omission still selects 30 days; explicit 7, 30 and 90 succeed. HTTP and UDS still refuse zero with 422.
The existing command regression failed before repair and now passes under the race detector.
Evidence: `overview-zero-red.log`, `overview-zero-green.log`, and `overview-zero-fixed*.json`.
