Goal (incl. success criteria):

- Remediate CodeRabbit review issues for PR `#70` end-to-end: export unresolved issues, triage each as VALID or INVALID, implement all valid fixes with tests, run full verification, create one remediation commit, resolve review threads, and confirm the exported summary is fully resolved.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the explicit `fix-coderabbit-review` workflow.
- Required skills in use: `fix-coderabbit-review`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `golang-pro`; `cy-final-verify` must gate commit/completion.
- Existing dirty worktree entries under `.agents/skills/fix-coderabbit-review/*` and `skills-lock.json` predate this task and must not be reverted or modified unless they are directly required.
- No destructive git commands without explicit user permission.
- Final task completion still requires `make verify`, even if the skill text mentions other verification commands.

Key decisions:

- Treat every CodeRabbit item as a hypothesis; only implement items that remain technically valid in the current tree.
- Keep this run scoped to PR `#70` review remediation; avoid unrelated cleanup in the already-dirty skill files.

State:

- Completed.

Done:

- Read repository instructions and the required skill files.
- Scanned existing ledgers for cross-agent awareness.
- Confirmed existing dirty worktree entries before starting.
- Exported PR `#70` review issues with authenticated GitHub access; the live export reports `6` unresolved review threads plus `1` unresolved outside-of-diff note.
- Triaged the unresolved items against the current tree:
  - `VALID` and requiring code changes: `issues/001`, `issues/002`
  - `VALID` but already satisfied in current tree: `issues/003`, `issues/005`, `issues/006`, `outside/002-*`
  - `INVALID` in current tree: `issues/004`
- Implemented the remaining live fixes in `internal/core/agent`:
  - introduced `ErrRuntimeConfigNil`
  - switched nil-config tests to `errors.Is`
  - wrapped the review-targeted tests in `t.Run("Should ...")` subtests
- Ran focused verification successfully: `go test ./internal/core/agent -count=1`
- Recorded technical dispositions directly in the exported review issue files.
- Ran `make verify` successfully before commit.
- Created the single remediation commit: `d9f7ed9` (`fix: resolve PR #70 review issues`).
- Resolved PR review threads `001` through `006` with the GitHub resolver script.
- Marked the non-thread outside-of-diff entry resolved locally and updated `ai-docs/reviews-pr-70/_summary.md` to `0` unresolved overall.
- Re-ran `make verify` successfully after the post-commit review-export updates.

Now:

- Prepare the final handoff with verification evidence and note the remaining uncommitted review-export updates.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-07-MEMORY-pr70-coderabbit.md`
- `ai-docs/reviews-pr-70/`
- `.agents/skills/fix-coderabbit-review/SKILL.md`
- `internal/core/agent/{registry_launch.go,registry_test.go,registry_validate.go}`
- Commands: `uv run ...pr_review.py 70 --hide-resolved`, `go test ./internal/core/agent -count=1`, `make verify`
