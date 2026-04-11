# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Add the capability enforcement layer and per-run `extensions.jsonl` audit writer required by task 04, with tests and clean repository verification.

## Important Decisions
- Use the existing extension capability and hook taxonomy from `internal/core/extension/manifest.go` as the single source of truth.
- Treat the PRD/techspec/protocol/ADR set as the approved design, so task 04 can go straight to implementation under `cy-execute-task`.
- Keep `CapabilityChecker` pure and move denial logging into a separate `WarnCapabilityDenied` helper so downstream hook and Host API paths can share the same authorization logic without coupling it to I/O.
- Keep the audit writer simple and synchronous behind a mutex, with one JSON line marshaled fully in memory per call and fsync deferred to `Close(ctx)`.

## Learnings
- `internal/core/extension/manifest_validate.go` already contains hook-family-to-capability mapping logic that should stay aligned with the runtime checker.
- `_protocol.md` section 5.2 defines one Host API method with no required capability: `host.prompts.render`.
- `internal/core/run/journal/journal.go` is the local precedent for append-only per-run writers and crash-safe truncation of partial tails.
- Gosec's taint analysis requires the audit log root path to be cleaned and resolved before the file open/stat calls.
- A helper-process kill test is practical for the crash-prefix requirement as long as the logger writes one complete JSON line per `Record` call and the test validates every persisted line.

## Files / Surfaces
- `internal/core/extension/manifest.go`
- `internal/core/extension/manifest_validate.go`
- `internal/core/run/journal/journal.go`
- `pkg/compozy/events/event.go`
- `internal/core/extension/capability.go`
- `internal/core/extension/audit.go`
- `internal/core/extension/capability_test.go`
- `internal/core/extension/audit_test.go`

## Errors / Corrections
- Pre-change baseline: no capability enforcement or audit logger exists yet in `internal/core/extension`.
- Corrected `AuditLogger.Record` to report lifecycle errors (`not open` / `closed`) before entry validation.
- Guarded `AuditLogger.Open` against reopen races while a prior close is still finishing.
- Switched the helper-process integration test to `exec.CommandContext` to satisfy lint policy.

## Ready for Next Run
- Task 04 implementation is complete, task tracking is updated, and the code changes were committed as `606ffb7` (`feat: add capability enforcement and audit log`).
- Verification evidence: `go test ./internal/core/extension -cover` (`81.8%`) and fresh `make verify` both passed after the final code change.
