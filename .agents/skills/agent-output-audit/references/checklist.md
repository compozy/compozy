# Agent Output Audit Checklist

Mark every item as complete before claiming the audit is done.

## Contract Discovery

- [ ] Root instructions and repository docs were read
- [ ] The canonical verify gate was identified or an explicit fallback was chosen
- [ ] The audit-output directory was resolved and `audit/` subdir created
- [ ] Compozy mode was detected (yes/no with `.compozy/tasks/<slug>/` path or "none")
- [ ] When in Compozy mode, `state.yaml`, `_techspec.md`, `_tasks.md` were read (state.yaml read-only)
- [ ] E2E support was determined (supported, manual-only, or blocked with evidence)

## Baseline Verification Gate

- [ ] Dependencies were installed with the repository-preferred command
- [ ] The baseline verification gate was run before any audit work
- [ ] Verification order followed fastest-first: lint, build, unit tests, integration tests
- [ ] Any pre-existing failures were isolated with evidence
- [ ] E2E command planning was recorded explicitly when it is separate from the umbrella gate

## Flaky Detection (Baseline)

- [ ] Each baseline failure was run in isolation 3-5 times on the same SHA before classification
- [ ] No PASS verdict was promoted from a single-retry rerun
- [ ] All `flaky-suspect` events were recorded in `SUITE HEALTH SNAPSHOT` with timestamp, attempts, retry outcome, and suspected category
- [ ] Flaky rate in the canonical suite is <2% (or documented as a known blocker)

## Task Implementation Audit

Skip this section only if no task, phase, PRD, tech spec, or implementation-plan artifacts exist.

- [ ] Task/phase/spec artifacts were discovered and listed
- [ ] Every task marked completed or claimed complete was compared against actual implementation files
- [ ] Every material requirement, subtask, deliverable, and success criterion was mapped to evidence
- [ ] Checked boxes and status fields were treated as claims, not proof
- [ ] Public behavior or automated tests were executed for each material completed task
- [ ] Incomplete completed tasks were marked `REOPEN` or linked to a `BUG-*` issue
- [ ] Large missing features were not silently passed as audit success
- [ ] The audit report includes a Task Implementation Audit section with per-task verdicts
- [ ] Task frontmatter `status:` was used as the declared status; `state.yaml` was read but not written
- [ ] When running in `.compozy/tasks/<slug>/`, `memory/qa-execution.md` was written with canonical sections **before** any frontmatter status was flipped (memory-precedes-status invariant)

## Independent Evaluator Stance

- [ ] The implementing agent's `memory/<phase>.md` artifacts were read **before** judging the task
- [ ] Transcript anomalies were classified (`genuine-failure` / `grader-bug` / `ambiguous-task` / `bypass-exploit`) in `memory/qa-execution.md` → `Errors / Corrections`
- [ ] No self-report (transcript success, `[x]` checkbox, memory `done`, frontmatter `status: completed`, PR description) was accepted as evidence

## AI Test-Hygiene Scan (RF-1..RF-6)

- [ ] Test diff scanned for `.skip` / `.only` / `xit` / `t.Skip` since the task baseline (RF-1)
- [ ] Assertions in modified test files verified to not weaken existing checks (RF-2)
- [ ] Mocks in Integration/E2E classified tests audited against TC `External Dependencies` (RF-3)
- [ ] Snapshot or gold-file changes justified by a documented requirement change (RF-4)
- [ ] Happy-path-only coverage flagged on P0/P1 tasks (RF-5)
- [ ] Test-implementation symbiosis bisected against Requirement → Test mapping (RF-6)
- [ ] Requirement → Test mapping table produced (`covers` / `weak` / `missing`) for every REOPEN candidate

## Final Verification

- [ ] The full verification gate was rerun after the last code change made during the audit
- [ ] Narrow E2E specs were rerun after the final code change when they were added or updated
- [ ] The canonical E2E command or covering subset was rerun when the repository supported E2E
- [ ] An audit report was produced from fresh evidence
- [ ] Blocked scenarios or missing prerequisites were disclosed explicitly

## Quality Gates

- [ ] Flaky rate <2% in canonical suite
- [ ] Zero `FAIL` from AI test-hygiene audit on P0/P1 tasks
- [ ] Zero `Critical` / `High` issues open
- [ ] Coverage delta ≥ baseline (no regression)
- [ ] Zero unresolved `flaky-suspect` on P0 flows
- [ ] Suite Health Snapshot populated in audit report
- [ ] All Quality Gates evaluated PASS/FAIL/N/A; a FAIL on any gate blocks unconditional final PASS
