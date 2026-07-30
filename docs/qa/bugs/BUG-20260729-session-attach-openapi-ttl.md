# BUG-20260729-session-attach-openapi-ttl: API clients were not told that long attach leases fail

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11 Resume an existing session, validate the attach lease
- **Scenarios:** RT-015
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

The public attach endpoint correctly rejected a lease above 24 hours, but its OpenAPI operation did
not declare the 400 response or the 86,400-second maximum. Generated clients therefore described a
narrower response set than the daemon actually returned and could not validate the TTL boundary.

## Reproduction

- **Charter:** CH-014 · **Tour:** Interrupt Tour
- **Environment:** desktop / HTTP + UDS + CLI / wifi-fast / en-US

1. Create an eligible live session through the public API.
2. Attach without a body and observe a 15-minute lease.
3. Attach another fixture with `{"ttl_seconds":999999}` and observe HTTP and UDS 400.
4. Inspect the published attach operation and generated TypeScript response union.

**Expected:** OpenAPI declares the 400 error and `ttl_seconds.maximum = 86400`.
**Actual:** The operation declared only 200/404/409/500, and `ttl_seconds` had no maximum.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach/openapi-attach-before.json`
- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach/openapi-regression-red.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach/runtime-overflow-regression-red.txt`
- Runtime 400 parity: `rt015-ttl-http.json`, `rt015-ttl-uds.json`, and their response headers in the same evidence directory.

## Fix

- **Root cause:** The handler owned private duration constants while the manually registered OpenAPI
  operation independently enumerated responses and left request bounds unconstrained. Codegen
  faithfully reproduced that incomplete registry. The first correction also converted an unbounded
  integer to `time.Duration` before comparing it with the maximum, allowing an extreme value to
  overflow negative and reach the Manager.
- **Correction:** Shared contract constants now drive runtime validation and the attach request schema;
  the handler rejects the raw integer before duration conversion; the declarative operation includes
  its 400 error; generated artifacts are refreshed; and the resume reference documents the same
  boundary.
- **Fix commit:** `351f3535`
- **Regression test:** `ShouldDescribeSessionAttachAndRecapContracts` in
  `internal/api/spec/spec_test.go`; `Should reject attach leases above the public maximum before the
  Manager CAS` in `internal/api/core/handlers_test.go`.

## Verification

- The OpenAPI regression failed before the correction with `missing 400 response`.
- The extreme-value regression failed before the runtime correction with HTTP 200 and a wrapped
  negative lease, proving that the Manager CAS was reached.
- Race-enabled coverage now passes for omitted/default, zero, negative, maximum, maximum-plus-one,
  and largest-integer TTL values; rejected requests never reach the Manager CAS.
- `make codegen-check` passes with the generated OpenAPI and TypeScript artifacts synchronized.
- The focused Web typecheck and Go lint for `internal/api/core` plus `internal/api/spec` pass.
- The correction shipped in `351f3535`; original-persona verification remains pending, so this bug is fixed rather than verified.
