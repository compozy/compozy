# BUG-<num>: <short-title>

**Severity:** Critical | High | Medium | Low
**Priority:** P0 | P1 | P2 | P3
**Type:** Functional | Performance | Security | Data | Crash | Hygiene
**Status:** pending | resolved | invalid | flaky-suspect | quarantined
**Reopens task:** <task_NN.md path or "none">

> Status values (aligned with cy-codex-loop `issue_NNN.md` frontmatter):
> - `pending` — issue is open and unresolved
> - `resolved` — fixed during this audit run and verified by re-run
> - `invalid` — triaged as non-actionable (not a defect, duplicate, environmental)
> - `flaky-suspect` — one run failed, retry passed; awaiting confirmation runs per `references/flaky-triage.md`
> - `quarantined` — confirmed flaky after diagnosis; isolated from merge gate but still monitored (requires named owner and fix-by date)

## Environment

- **Build:** <version or commit>
- **OS:** <operating system if relevant>
- **Compozy slug:** <.compozy/tasks/<slug>/ or "n/a">

## Summary

<Describe the observable failure (or audit finding) in one short paragraph. For RF-* findings, name the red flag ID.>

## Reproduction

```bash
<exact command, scan, or sequence>
```

Observed before the fix:

- <observable result>

## Expected

<Describe the correct behavior or the Success Criterion that was not met.>

## Root cause

<Describe the actual source of the failure, not the symptom. For RF-2, name the original strict assertion that was weakened.>

## Fix

<Describe the production change (or test restoration) that fixed the root cause.>

## Verification

- <narrow reproduction rerun>
- <broader regression or full gate rerun>
- <Requirement → Test mapping table updated, if applicable>

## Impact

- **Users Affected:** <all / subset / specific role>
- **Frequency:** <always / sometimes / rarely>
- **Workaround:** <describe or "none">

## Automation Follow-up

- **Red Flag ID:** <RF-1..RF-6 or "n/a">
- **Verdict:** FAIL | PARTIAL | PASS
- **Required:** Yes | No
- **Status:** Added | Pending | Blocked | N/A
- **Spec / Command:** <path, suite, or command>
- **Notes:** <rationale or blocker>

## Flake Evidence (when Status is `flaky-suspect` or `quarantined`)

- **Failure Pattern:** consistent | intermittent | order-dependent | env-only
- **Reproducibility Rate:** <e.g. 3/10 runs>
- **Suspected Category:** async-wait | concurrency | order-dep | external | non-determinism | other
- **Owner:** <named person, not team>
- **Fix-by Date:** <YYYY-MM-DD>

## Transcript Anomaly (when applicable)

- **Classification:** genuine-failure | grader-bug | ambiguous-task | bypass-exploit
- **Evidence path:** <.compozy/tasks/<slug>/memory/<phase>.md line or section>

## Related

- Task: <task_NN.md path>
- Memory: <memory/qa-execution.md section>
