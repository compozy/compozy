# BUG-20260729-overview-json-parity: Agents receive different overview JSON from CLI and API

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-daemon-schema, compare agent-manageable overview surfaces
- **Scenarios:** RT-observe-overview-cli
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** n/a

## Summary

An agent requesting the machine-readable Home overview through the CLI received an extra
`resolution_source` field that HTTP and UDS did not return. Strict consumers therefore could not
treat the three advertised surfaces as one stable payload.

## Reproduction

- **Charter:** CH-untested-066-operate-daemon-schema-ada · **Tour:** Garbage Tour
- **Environment:** isolated macOS lab, desktop / wifi-fast / en-US

1. Start the candidate daemon in the charter's isolated workspace.
2. Read the 30-day overview through `compozy observe overview -o json`, HTTP, and UDS.
3. Remove only timestamp-derived fields and compare the three payloads.

**Expected:** CLI emits the raw `observe-overview/v1` payload and matches the `overview` value from
HTTP and UDS.
**Actual:** CLI inserted `resolution_source: flag`; HTTP and UDS matched each other without that
field.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/003-structured-read-contracts`
- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/004-observe-overview-retest`

## Fix

- **Root cause:** The generic CLI output writer injected workspace-resolution metadata into every
  JSON object after the observe command had already selected the public overview payload. The
  canonical test decoded into a struct, so Go silently ignored the unexpected key.
- **Fix commit:** pending completion gate
- **Regression test:** `internal/cli/observe_test.go` — failed before the production change and
  passes after it; the real CLI/HTTP/UDS replay also matches after timestamp normalization.

## Verification

- **Retested:** pending fix commit
- **Result:** The staged candidate replay passes, but registry verification remains pending until
  the governed fix commit exists.
