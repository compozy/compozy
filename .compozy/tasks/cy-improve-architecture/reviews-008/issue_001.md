---
provider: manual
pr:
round: 8
round_created_at: 2026-07-14T06:43:59Z
status: resolved
file: extensions/cy-improve-architecture/evals/skill_e2e_test.go
line: 74
severity: medium
author: claude-code
provider_ref:
---

# Issue 001: Isolated E2E home exceeds Darwin socket limit

## Review Comment

The model-backed subtest now uses `t.TempDir()` as `COMPOZY_HOME`. `compozy exec` bootstraps the
home-scoped daemon, whose Unix socket is derived as `<COMPOZY_HOME>/daemon/daemon.sock`. On Darwin,
the long testing temp root plus the subtest name can produce a socket path longer than the platform's
Unix-domain `sun_path` limit, so the opt-in evaluation can fail while starting or connecting to the
daemon before it audits either fixture.

The normal verification suite cannot detect this because the ACP-backed test is skipped unless its
environment gate is enabled. The repository's daemon tests already avoid the same failure mode by
allocating daemon homes beneath a short base such as `/tmp`.

Keep the isolated-home boundary, but create it through a cleanup-managed short-path helper, using a
short system temp base on Unix and an appropriate temporary directory on Windows. Add a focused
assertion that the derived daemon socket path stays within the supported platform limit.

## Triage

- Decision: `VALID`
- Root cause: `TestAuditSkillProducesInspectableArtifacts` creates its isolated `COMPOZY_HOME` with `t.TempDir()`. The daemon resolves its socket as `<home>/daemon/daemon.sock`; Darwin supports at most 103 path bytes, so the long test root can prevent the opt-in ACP evaluation from starting.
- Fix: allocate each evaluation home beneath `/tmp` on Unix (falling back to the system temporary directory when unavailable), retain cleanup-managed isolation, and assert the real resolved daemon socket path remains within the Darwin-compatible limit. Windows continues to use its system temporary directory and does not require a Unix-socket limit assertion.
- Resolution: `newShortEvaluationHome` now supplies the model-backed E2E home, preserves per-subtest cleanup, and the focused test resolves the canonical daemon socket path before enforcing the 103-byte Darwin-compatible ceiling. `make verify` passed after the implementation.
