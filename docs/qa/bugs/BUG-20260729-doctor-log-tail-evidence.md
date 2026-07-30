# BUG-20260729-doctor-log-tail-evidence: Available log-tail diagnostic omitted evidence

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-daemon-schema, inspect bounded runtime diagnostics
- **Scenarios:** RT-002
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated HTTP, UDS, and CLI Doctor replay

## Summary

`GET /api/doctor` returned an otherwise valid `doctor.logs.tail` item for an available log-tail
capability, but omitted its structured `evidence` object. Every other item in the same live response
carried evidence, and the unavailable log-tail branch already reported its authoritative status.

## Reproduction

1. Start a daemon whose settings service advertises log-tail support.
2. Read `GET /api/doctor`, the same route over UDS, and `compozy doctor -o json`.
3. Locate the `doctor.logs.tail` item in each payload.

**Expected:** The item includes `evidence.status: "available"`, matching the capability state that
selected its code and severity.
**Actual before the fix:** JSON `omitempty` removed the nil evidence map, leaving the successful
log-tail item without structured evidence.

## Evidence

- Pre-fix reproduction and rebuilt three-surface retest:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/025-runtime-doctor`.

## Fix

- **Root cause:** The available branch of `logTailDiagnosticItem` omitted the `WithEvidence` option;
  the unavailable branch already attached the same authoritative status field.
- **Correction:** Both branches now attach `status` evidence from `LogTailStatusPayload` through the
  canonical redacting diagnostic constructor.
- **Fix commit:** `351f3535`
- **Regression owner:** `internal/api/core/handlers_test.go`,
  `TestDoctorLogTailDiagnosticIncludesCapabilityEvidence/Should expose the available log-tail capability status as evidence`.

## Verification

- The canonical route-level regression failed before the production change and passes under
  `go test -race` afterward; the complete `internal/api/core` race suite also passes.
- `make lint` and `make build` pass.
- The rebuilt daemon passed 38/38 live assertions across HTTP, UDS, and CLI, including filter
  behavior, structured evidence completeness, and `evidence.status: "available"`.

## Compozy Impact Audit

- **Native tools:** no impact; checked the Doctor ownership statement in
  `skills/compozy/references/native-tools.md` and the `compozy__*` registry. Diagnostics remain
  CLI/HTTP/UDS-owned and no native tool ID, descriptor, schema digest, or capability gate changed.
- **Extensibility and hooks:** no impact; checked extension, hook, skill, bundle, registry, bridge,
  MCP, and config-lifecycle surfaces. The change only completes one existing diagnostic projection.
- **Workspace data isolation:** no impact; log-tail availability is daemon-global, and no global,
  workspace, session, or agent datum changed ownership or propagation.
- **Official Compozy skill:** no update required; checked
  `skills/compozy/references/runtime-operations.md` and `native-tools.md`. They already describe
  Doctor as the structured CLI/HTTP/UDS diagnostic surface without pinning the previously missing
  field.
