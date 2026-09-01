# QA Run Report — 2026-09-01 — issue-512-loop-worktree-tools

- **Scope:** Issue 512 — selected Loop Worktrees scope extension tools and Web authors only root/worktree without losing API/CLI values
- **Cadence tier:** targeted
- **Build:** `1881a9f54` + working tree · **Environment:** isolated lab, `http://127.0.0.1:62770`
- **Started:** 2026-09-01T13:33:55Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-worktree-tool-root |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-web-environment-authoring |

## Flows in Scope

- `J-isolated-task-loop-execution` — Run task and Loop work in isolated environments (`../journeys/J-isolated-task-loop-execution.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-worktree-tool-root | J-isolated-task-loop-execution / LP-loop-environment-resolution | Ada | Feature Tour | Pass | | |
| 2 | CH-loop-web-environment-authoring | J-isolated-task-loop-execution / LP-worktree-web-loop-environment | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-worktree-tool-root — Ada

- **Ran:** 2026-09-01T13:41Z → 13:45Z (box respected: yes)
- **Findings:** A root-scoped canary returned `No task set matched`; after selecting ready Worktree
  `wt_d7e6c291e8cae07e`, the same extension action completed and returned the sole task from
  `project-worktree/.compozy/tasks/issue-512-qa/task_01.md`. CLI, HTTP, Web, and runtime records
  agreed for `looprun-4047102765205c5a`, including after reload.
- **Bugs filed/updated:** none
- **Scenarios settled:** LP-loop-environment-resolution → pass

### CH-loop-web-environment-authoring — Bruno

- **Ran:** 2026-09-01T13:43Z → 13:48Z (box respected: yes)
- **Findings:** The Run form offered only Inherit, Workspace root, and Named worktree. A
  CLI-authored directory rendered as `Directory (read-only)`, its input had the browser
  `readOnly` property, and saving the unrelated Human approval gate preserved the exact directory.
  A CLI-authored per-run value then rendered as `Per-run (read-only)`.
- **Bugs filed/updated:** none
- **Scenarios settled:** LP-worktree-web-loop-environment → pass

## What Was Fixed

No QA findings fixed in this run.

## Paper Cuts

None.

## Runtime Errors Observed

- The first-run wizard remained on `Loading Continue` within the session patience window after the
  default Codex model was selected. The session ended without retries; the supported
  `compozy onboarding complete` command established the precondition for a fresh browser session.
  This did not recur in either in-scope Loop journey.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A one-action Loop is the cleanest behavioral probe for this regression: it isolates extension
  workspace resolution from provider availability while still using the published Loop, CLI,
  HTTP, Web, and extension subprocess surfaces.
- The strongest preservation proof pairs the read-only Web control with a structured CLI read
  after an unrelated Web save.

## Final Status

- **Behavioral sessions:** PASS — 2/2 targeted scenarios passed.
- **Evidence:** `docs/qa/evidence/2026-09-01-issue-512-loop-worktree-tools/` plus the isolated lab's
  `qa/evidence/issue-512-loop-worktree-tools/` structured CLI/API reads.
- **Evidence audit:** PASS — strict audit reported 0 blockers and 0 warnings.
- **Local gate:** PASS — all classified Go and Web lanes completed successfully.
- **Teardown:** PASS — `qa/teardown.json` records `"clean": true` with no surviving lab processes.
- **QA verdict:** PASS — exact-head PR CI remains the repository delivery gate outside this QA run.
