---
provider: coderabbit
pr: "151"
round: 1
round_created_at: 2026-05-14T00:24:33.853673Z
status: resolved
file: internal/daemon/run_manager.go
line: 1176
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6B7N8T,comment:PRRC_kwDORy7nkc7BA5WM
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_ | _🏗️ Heavy lift_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _🏗️ Heavy lift_

**Don't drop the resumed row's start-state signal.**

At Line 1168 `resumeExistingExecRun` can already insert or reset a persisted exec row to `starting`, but Line 1173 always returns `createdRun=false` on that success path. If `openRunScopeForStart` fails right after, `startRun` skips both cleanup and terminal-state mirroring, so the resumed run can be left stranded in `starting`. Preserve whether the resume path touched the row and use that to either roll it back or mark it failed before returning.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/daemon/run_manager.go` around lines 1164 - 1176, The
resumeExistingExecRun success path currently returns createdRun=false which
loses the fact the persisted row was touched; change the return to propagate the
`ok` flag returned by `resumeExistingExecRun` (i.e., return row, ok, nil) so
callers like `openRunScopeForStart`/`startRun` know the row was resumed and can
run rollback/terminal-state mirroring; ensure the code path that calls
`resumeExistingExecRun` uses the `ok` boolean instead of a hardcoded false when
deciding cleanup or failure-marking.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `prepareRunRow` currently calls `resumeExistingExecRun(...)`, but when that succeeds it returns `createdRun=false` unconditionally.
  - On the resumed-exec path, `resumeExistingExecRun` may either insert a fresh global row or reset an existing row back to `starting`. If `openRunScopeForStart` then fails, `startRun` returns before `failStartRun`, so the touched row can remain stranded in `starting`.
  - Fix approach: preserve resume-path state in `prepareRunRow`/`startRun`, and on scope-open failure route resumed exec rows through failure cleanup instead of the generic “delete only newly created rows” branch; add run-manager coverage for the resumed exec failure path.
  - Resolution: resumed exec rows now preserve both “resumed” and “created” state through `prepareRunRow`, and scope-open failures route through `failStartRun`; new run-manager coverage exercises both inserted and existing resumed rows.
