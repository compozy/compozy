# QA Run Report — 2026-09-10 — Dream health reporting

- **Scope:** Issue #603; effective Dream role diagnostics across memory health, native tools, and Settings.
- **Cadence tier:** targeted
- **Build:** branch based on `ed7f2d7adc2d7ec38071677d28a2d6e87a019c28`; local changes under validation.
- **Environment:** isolated macOS daemon, CLI, HTTP, and UDS. No provider sessions or model calls are required.
- **Status:** pass (scoped diagnostic verification)

## Personas and flows

Rafa, desktop / local network / en-US, follows the health slice of [J-digest-sessions-into-memory](../journeys/J-digest-sessions-into-memory.md) under [CH-untested-valid-019-digest-sessions-into-memory-rafa](../charters/CH-untested-valid-019-digest-sessions-into-memory-rafa.md). Role diagnostics provide the adjacent canary. Consolidation execution and the unchanged Web rendering are outside this diagnostic-only walk.

## Session Matrix & Results

| Session | Journey slice | Result | Evidence |
| --- | --- | --- | --- |
| Rafa health inspection | Four opt-in combinations; role/health parity | Pass | `TestDaemonE2EMemoryDreamHealth`, four real daemon configurations |
| Rafa configuration inspection | Workspace overrides and live role changes | Pass | `TestDaemonE2EMemoryDreamHealthLiveScope`, CLI writes plus independent HTTP/UDS and Settings reads |

## Session Debriefs

The health workflow was replayed through the canonical real-daemon integration harness. This is automated public-interface evidence, not an additional manual or browser walk. The factory-off, memory-only, role-only, and both-on states report Dream enabled as false, false, false, and true across CLI/HTTP/UDS. User role writes false/true/false affect fresh global health and Settings reads immediately, while an enabled workspace override stays enabled. No session prompt or dream trigger is issued.

The original-base binary reproduces the defect: memory enabled, role disabled, role status false, health true with status ok. The same assertion passes against the corrected binary. Profile context and unknown-profile error propagation are covered in the role projection suite; public profile selection semantics were not changed.

The broader browser portion of MS-011 keeps its historical blocked verification status; this run settles only the changed diagnostic slice.

## What Was Fixed

Health read models used the memory trigger's master switch as the Dream role's enabled state. They now consume the non-invoking role status projection, preserving scoped resolution and explicit failures. Execution eligibility is unchanged.

## Validation

Focused core, HTTP, UDS, and Settings tests passed with `CGO_ENABLED=1 go test -race`. The first new integration run reached matching HTTP/UDS states but its CLI used the repository cwd instead of the harness workspace; the fixture now uses `RunJSONInDir` with the registered isolated workspace. No production assertion was weakened.

The initial local gate inherited `COMPOZY_INTERNAL_RESTART_OPERATION_ID` from the host. Its daemon fixtures had no matching restart record and waited for readiness. An isolated diagnostic exposed that startup error; the temporary diagnostic edit was removed. The unchanged lifecycle suite passes when that environment variable is unset. The final gate is run with `env -u COMPOZY_INTERNAL_RESTART_OPERATION_ID make gate`; no host daemon or persisted state is changed.

## Limits and final status

The scoped real-run verification and `env -u COMPOZY_INTERNAL_RESTART_OPERATION_ID make gate` passed. The gate reports zero lint issues and all affected Go suites passed, including the daemon race suite. PR delivery separately requires current-head CI and both external reviews. This macOS pass does not claim a local Linux run or live model execution.

## Reproducible evidence

- Base reproduction: build the five changed production files from the base commit with a Go build overlay, then run the same integration case against that binary. `TestDaemonE2EMemoryDreamHealth/Should_report_memory_enabled_without_dreaming` fails on the base with `DreamEnabled:true` versus role `Enabled:false`.
- Fixed real run: `CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2EMemoryDreamHealth' -count=1 -v` passes the four-state matrix and the live workspace/Settings comparison.
- Focused owning tests: core health, HTTP/UDS health, Settings health, native tool health, and role projection suites pass with race detection.
- Test-shape checker: touched core/daemon suites pass. Transport suites retain pre-existing out-of-scope inline tests; the touched health case now uses a canonical `Should` subtest.
