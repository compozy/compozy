---
provider: manual
pr:
round: 9
round_created_at: 2026-07-14T07:28:52Z
status: resolved
file: extensions/cy-improve-architecture/evals/skill_e2e_test.go
line: 76
severity: high
author: claude-code
provider_ref:
---

# Issue 001: E2E leaves detached daemons across isolated subtests

## Review Comment

Each model-backed fixture subtest creates a different `COMPOZY_HOME` and runs `compozy exec`.
That command auto-starts a detached, home-scoped daemon, but the evaluator neither requests an
ephemeral HTTP port nor stops the daemon afterward. With `COMPOZY_DAEMON_HTTP_PORT` unset, every
isolated daemon resolves to the shared default port `2323`.

If a developer daemon already owns that port, the first fixture cannot start. Otherwise the first
fixture leaves its detached daemon listening on `2323`, and the second fixture's different home
cannot discover or reuse it, so the second daemon fails to bind. `newShortEvaluationHome` then only
removes the first daemon's home directory, unlinking its discovery state while leaving the process
running. The documented dual-fixture ACP evaluation therefore cannot reliably complete and can leak
an undiscoverable background process.

Make every evaluation daemon test-owned: set `COMPOZY_DAEMON_HTTP_PORT=0` in the child environment
to request an ephemeral port, then register cleanup that invokes `compozy daemon stop --force` with
the same `COMPOZY_HOME`, waits for the stopped state, and only then removes the home. Add lifecycle
coverage proving two sequential isolated executions do not share a port and leave no daemon behind.

## Triage

- Decision: `VALID`
- Root cause: `auditCommand` creates one daemon per isolated `COMPOZY_HOME`, but its child environment leaves `COMPOZY_DAEMON_HTTP_PORT` unset. Each daemon therefore resolves to the shared default port (`2323`). `newShortEvaluationHome` removes the home at test cleanup without a preceding daemon shutdown, leaving the daemon process and its listener alive after discovery data is removed.
- Fix approach: require an ephemeral daemon port in every evaluation child environment; register test cleanup to run `compozy daemon stop --force` using that same home, condition-poll for `stopped`, then allow the pre-registered home cleanup to remove the directory. Extend the existing real ACP evaluation to retain both sequential fixture daemons until assertions prove distinct non-default ports; cleanup polling proves each daemon has stopped before its home removal.
- Verification: `go test ./extensions/cy-improve-architecture/evals -count=1` passed 43 tests. Repository gate `make verify` passed with exit code 0 after formatting, linting, tests, and builds.
