# BUG-20260729-loop-native-error-semantics: Native Loop errors discarded actionable recovery state

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-07 native Loop definition management
- **Scenarios:** TA-075; TA-076
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

An agent could not recover deterministically from a stale Loop write because the native error omitted
the current version and mislabeled the conflict as a registry identity collision. Deleting a read-only
Loop was separately classified as invalid JSON input, while HTTP and UDS returned 400 despite the
declared 403 contract. Stateful native controls had the same adapter defect: repeating Pause on an
already paused run returned generic `schema_invalid` even though HTTP and UDS preserved the domain's
`invalid_status_transition` reason.

## Reproduction

- **Charter:** CH-untested-008-07-ada · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / native tools + HTTP + UDS / en-US

1. Create a workspace-authored Loop at version 1, publish version 2, then call
   `compozy__loop_create` with `expected_version: 1`.
2. Call `compozy__loop_delete` for bundled marketplace Loop `review-and-fix`.
3. Repeat the read-only delete through HTTP and UDS and read both definitions afterward.
4. Pause a real Loop at a generation boundary, then repeat native Pause and compare the result with
   HTTP and UDS.

**Expected:** stale native CAS returns the current version with a Loop-specific conflict reason;
read-only mutation is denied consistently and neither operation changes state.
**Actual:** stale native CAS returned `conflicted_id` without `current_version`; native read-only
delete returned `schema_invalid`; HTTP and UDS returned 400.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-catalog-native-20260729-20260729-185423-303044-lab/qa-artifacts/qa/evidence/050-loop-catalog-native/native-create-stale-separated.stdout.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-catalog-native-20260729-20260729-185423-303044-lab/qa-artifacts/qa/evidence/050-loop-catalog-native/native-delete-read-only.stdout.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-catalog-native-20260729-20260729-185423-303044-lab/qa-artifacts/qa/evidence/050-loop-catalog-native/http-delete-read-only.json`
- Repaired replay: `ta075-retest-assertions.json` and the adjacent `ta075-retest-*` receipts.
- Stateful control mismatch and repaired replay:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-controls-20260729-20260729-205221-682163-lab/qa-artifacts/qa/evidence/053-loop-native-controls/ta076-negative-pause-paused-native.json`
  and `ta076-negative-pause-paused-native-retest.json`.

## Fix

- **Root cause:** The native Loop adapter classified every error only through HTTP status, discarding
  the typed CAS payload. Read-only source validation reused the generic Loop validation sentinel, so
  transports and native tools interpreted an authorization boundary as malformed input.
- **Correction:** A Loop read-only sentinel now maps to HTTP/UDS 403 and native
  `tool_denied/loop_source_immutable`. Typed CAS conflicts map to
  `tool_conflict/loop_version_conflict` with `partial_result.structured.current_version`. The same
  adapter now preserves typed control reasons such as `invalid_status_transition` and
  `terminal_loop_run` before applying status-based fallbacks.
- **Fix commit:** `103192e4`
- **Regression test:** `internal/daemon/native_loop_tools_test.go`,
  `internal/daemon/loop_resources_test.go`, `internal/api/core/errors_test.go`, and
  `internal/api/core/loops_test.go`.

## Verification

- Red-before regressions reproduced both native classifications and the transport 400.
- Focused owner suites pass under `-race`; scoped Go lint, codegen-check, generated Web typecheck,
  skill bundle tests, and candidate build pass.
- The rebuilt daemon returned both new reason codes, preserved `current_version: 2`, aligned HTTP and
  UDS on byte-equivalent 403 bodies, and preserved both the workspace definition and marketplace
  source.
- The rebuilt stateful-control replay returned native
  `tool_invalid_input/invalid_status_transition` for repeated Pause, matching the domain reason
  exposed by HTTP and UDS; positive Pause, Resume, and Stop retained their successful contracts.
- **Retested:** 2026-07-29 in the original isolated Loop catalog and stateful-control labs.
- **Result:** Pass. Governed fix commit `103192e4`; original-persona native/HTTP/UDS replays green.
