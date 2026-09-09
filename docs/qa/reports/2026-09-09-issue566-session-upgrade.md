# QA Run Report — 2026-09-09 — Session profile upgrade and diagnostics

- Scope: #566, previous-release persisted profiles, unreadable warning cadence, and doctor.
- Tier: targeted, CLI/API/runtime, no provider execution required.
- Build: dedicated `fix/issue-566-session-upgrade-warnings` checkout based on `eacd0cb53b9cf4b72fb61fb5db6a75868a3a48ce`.
- Lab: `compozy-issue566-session-upgrade-20260909-160557-598361-lab`; HTTP port 56656 and a private runtime home from its bootstrap manifest.
- Runtime QA and affected delivery gate: PASS. External PR review remains in progress.

## Session matrix

| Journey | Persona | Mission | Status |
| --- | --- | --- | --- |
| RT-002 | Dora, runtime operator | Inspect unreadable counts through CLI and HTTP; verify bounded evidence and recovery | Pass |
| RT-022 | Ada, session operator | Reopen beta.21 session metadata and read retained history after restart | Pass |

## Evidence collected

- The new canonical-profile regression failed before the production fix with `store: unsupported session creation profile version 4`.
- Historical fixture generated from beta.21 profile/hash code, not current hashing. Metadata read/rewrite and global SQLite close/reopen preserve original profile bytes and identity triples. Tampered and future-version fixtures remain rejected.
- Twelve repeated list/scan calls emit one summary. A changed failure outside the five-sample window emits immediately; the unchanged set emits again after five minutes. Repair followed by recurrence emits again.
- Doctor runner and HTTP handler tests verify severity, count, sample, and filter projection.

## Cross-surface impact

- Native tools: no `compozy__*` IDs, schemas, or toolsets change. Session readers retain historical witnesses; CLI/HTTP/UDS doctor gains `runtime.session_metadata` through the existing `DiagnosticItem` envelope.
- Extensibility/hooks/config: no hook, extension, bridge, SDK, credential, or config contract changes. Dreaming defaults and gate thresholds are unchanged. Warning state belongs to each scanner instance.
- Workspace isolation: profiles remain globally content-addressed with their original workspace identity; metadata retains session ownership and all digests. Doctor is an operator diagnostic, exposing only bounded, redacted identifiers/errors; no agent read grants change.
- Official skill: `skills/compozy/references/runtime-operations.md` documents the probe, supported versions, bounded warning behavior, and preserved-data recovery.
- Web/Docs: session visibility improves through existing backend readers; no Web component change. Troubleshooting and release migration notes document the supported upgrade and diagnostics.
- Compatibility: user-state boundary accepts v4 and v5 without rewriting immutable witnesses. New profiles remain v5. No SQL shape change or migration number; no public DTO shape changes or deprecated aliases.

## Limits

The synthetic fixture covers creation-profile compatibility, not every database shape from a complete beta.21 home. No claim is made about reproducing the issue reporter's 90 MB/h or CPU attribution. Doctor checks metadata and catalog witnesses; it does not validate every session event database.

## Runtime session debrief

Dora inspected `runtime.session_metadata` through the real HTTP endpoint and UDS-backed CLI.
Both returned 9 checked sessions, 8 deliberately unreadable fixtures, 5 samples and 3 omitted.
After withdrawing only those injected fixtures, the probe returned zero failures; `--quiet`
omitted the healthy item. Ada listed `sess-beta21`, inspected its runtime selection, and read its
two retained events. After stopping and restarting the real daemon, the same history JSON was
byte-identical. The original `meta.json` SHA-256 also remained unchanged.

Twelve list calls over a measured 60-second interval produced one metadata warning summary,
not one warning per failed session per call. Memory did not run in this interval; its warning
recovery/cadence is covered by the owning service test, with defaults left unchanged.

Artifacts are under the lab's `qa-artifacts/qa/evidence/`: `doctor-healthy-cli.json`,
`doctor-unreadable-http.json`, `doctor-unreadable-cli.json`, `doctor-recovered-cli.json`,
`doctor-restart-http.json`, `doctor-quiet.json`, `session-list.json`, `session-status.json`,
`history.json`, `history-restart.json`, `warning-window.json`, and the before/after metadata hashes.
Startup also reported unrelated user-skill frontmatter/verification warnings; no provider was
started and no operator configuration or credentials were changed.

## Verification commands

- `CGO_ENABLED=1 go test -race ./internal/store ./internal/store/globaldb -run 'TestSessionMetaPreviousReleaseWitness|TestGlobalDBPreviousReleaseCreationProfile' -count=1` — PASS.
- `CGO_ENABLED=1 go test -race ./internal/session ./internal/memory -run 'TestManagerUnreadableMetadataWarnings|TestServiceMetadataWarningRecovery' -count=1` — PASS.
- `CGO_ENABLED=1 go test -race ./internal/doctor ./internal/api/core -run 'TestSessionMetadataProbe|TestDoctorSessionMetadataProbe' -count=1` — both package suites passed (the API case was rerun after correcting fixture setup).
- `CGO_ENABLED=1 go test -race -tags=integration ./internal/store/globaldb ./internal/session -run 'TestGoalSessionCreationIdentityIntegration|TestManagerIntegrationUsesRealSQLitePerSessionDB' -count=1` — PASS with real SQLite dependencies.
- `go build -o .cache/issue566/compozy ./cmd/compozy` — PASS; this binary ran the CLI/HTTP/runtime walk.
- Test-shape checker: zero new findings compared with the unchanged base of each touched test file. Existing legacy cases were not rewritten.
- `make gate` — PASS for all affected lanes, with zero lint findings and race-enabled tests. Historical fixture paths were subsequently changed to `.json.golden` with identical bytes; the final gate rechecks those owning suites. The initial formatter-only failure was corrected with pinned golangci-lint 2.13.1.

The strict lab audit passed with no blockers. Targeted teardown reported
`TEARDOWN_ALL_CLEAN=true`; the lab's `qa-artifacts/qa/teardown.json` contains `clean: true`.
Only this lab's registered daemon was stopped; no machine-wide reap ran.
