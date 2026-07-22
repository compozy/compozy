---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: resolved
file: internal/cli/validate_tasks.go
line: 188
severity: medium
author: claude-code
provider_ref:
---

# Issue 007: Direct Task Group validation ignores the canonical plan mapping

## Review Comment

The direct `--tasks-dir` validation path treats any immediate child of `_task_groups/` as a Task Group suite and trusts the suite's own `workflow` field as the expected identity. It verifies only that the initiative segment matches the containing directory; it does not load `_task_groups.md`, confirm the stable ID exists, or confirm that the selected graph node's declared `directory` equals `tasksDir`. An orphan directory—or `_task_groups/002-api/_tasks.md` claiming `demo/TG-001`—can therefore pass validation even though runtime resolution maps that stable ID elsewhere.

Resolve the manifest reference through the canonical Task Group plan and compare the resolved operational directory with the requested directory before validating the suite. Add cases for orphan directories, unknown IDs, and a manifest whose valid stable ID belongs to a different declared directory.

## Triage

- Decision: `VALID`
- Notes: `validateDirectTaskGroupSuite` used the child `_tasks.md` manifest's own
  `workflow` value as `ExpectedWorkflow`, then checked only that the reference parsed
  and named the containing initiative. It never resolved that reference through the
  initiative's canonical `_task_groups.md` plan, so missing plans, unknown stable IDs,
  and IDs mapped to another declared directory could all be reported as valid. The fix
  resolves the manifest reference with `taskgroups.TargetResolver`, compares the
  resolver's containment-checked Task Group directory with the requested directory,
  and reports mapping failures as validation issues. Regression coverage exercises
  orphan directories, unknown IDs, and valid IDs mapped to a different directory.
- Verification: `make verify` passed after the code and regression-test changes.
