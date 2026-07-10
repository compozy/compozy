---
name: agent-output-audit
description: >-
  Independent audit of AI-implemented work — certifies a completed task actually
  did what it claims, checking files, diffs, tests, and CI rather than the
  agent's self-report. Flags skipped or weakened tests, mock-hidden integration,
  snapshot drift, happy-path-only coverage, flaky retries, and status/evidence
  mismatches. Use when validating completed Compozy tasks, AI-authored PRs, or
  codex-loop iterations. Not for real-user, persona, or journey QA — use
  qa-execution for those.
argument-hint: "[audit-output-path]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Agent Output Audit

You are the **independent evaluator**. Answer one question — *"Did the implementing agent actually do what `task_NN.md` says it did?"* — from files, public behavior, tests, and CI. A self-report is not evidence. (Whether a real user can succeed at the product is `qa-execution`; run both on a Compozy slug and keep their outputs separate.)

## Step 1: Discover the Repository Verification Contract

1. Read root instructions, repository docs, and CI/build files before running commands.
2. Run `python3 scripts/discover-project-contract.py --root .` to surface candidate install/verify/build/test/lint/start commands and E2E signals.
3. Prefer repository-defined umbrella commands (`make verify`, `just verify`, CI entrypoints) over language defaults. When discovery surfaces more than one plausible gate or mixes ecosystems, read `references/project-signals.md` before choosing, and state the tie-breaker.
4. Read `references/e2e-coverage.md` before classifying any flow's coverage.
5. Resolve the audit artifact directory: the `audit-output-path` argument if given, else repository conventions, else `/tmp/agent-output-audit-<slug>`. Create its `audit/` subdirectory; store all bugs and reports under `<audit-output-path>/audit/`.
6. **Detect Compozy mode.** If `.compozy/tasks/<slug>/` exists, record the slug and:
   - Read `state.yaml` **read-only** — `scripts/update-state.py` owns its mutation per the cy-codex-loop contract.
   - Read `_techspec.md` (deliverable source of truth) and `_tasks.md` (task roster) when present.
   - List every `task_NN.md` and capture its frontmatter `status:` (`pending` | `in_progress` | `completed`). When frontmatter disagrees with `state.yaml`, frontmatter is the source of truth.
   - Note the memory slot `.compozy/tasks/<slug>/memory/qa-execution.md` — Step 5 writes it before any status flip.

## Step 2: Run the Baseline Verification Gate

1. Install dependencies with the repository-preferred command.
2. Run the canonical gate once before any audit work, fastest-first: lint and type-check → build → unit tests → integration tests.
3. If the E2E command is separate from the umbrella gate, decide whether to run it now or after runtime prerequisites are ready, and record that plan.
4. On a baseline failure, read the first failing output and determine whether it is pre-existing or introduced by current work. Exclude a failure from audit scope only after a clean reproduction proves it unrelated.
5. **Flaky-failure protocol.** Before classifying any baseline failure, run the failing test in isolation 3-5 times on the same SHA. If it passes at least once without a code change, record it as `flaky-suspect` in the `SUITE HEALTH SNAPSHOT` (test name, attempts, retry outcome, suspected category) rather than promoting it to PASS. Read `references/flaky-triage.md` before assigning a suspected category or proposing a quarantine.

## Step 3: Audit Task Implementations

Skip this step only when no task, phase, PRD, tech spec, or implementation-plan artifacts exist.

1. Read `references/independent-evaluator-protocol.md` in full before forming any verdict — it owns what does and does not count as evidence, and the transcript classification (`genuine-failure` / `grader-bug` / `ambiguous-task` / `bypass-exploit`). In Compozy mode, read the implementer's `memory/<phase>.md` artifacts and record anomaly classifications in `memory/qa-execution.md` → `Errors / Corrections` before judging the task.
2. Summarize each `task_NN.md` and its body into a Task Implementation Matrix (columns mirror cy-codex-loop frontmatter):
   - `task_path`, `declared_status` (literal frontmatter `status:`)
   - `title`, `type`, `complexity`, `dependencies` — mirrored from frontmatter
   - `techspec_deliverable` — linked `_techspec.md` section when present
   - Requirements, subtasks, checklist items, success criteria, dependent files
   - `implementation_evidence` — files, modules, routes, commands, migrations, seeds, tests
   - `verification_evidence` — commands executed, exit codes, output summaries
   - `qa_verdict` — `PASS` | `PARTIAL` | `FAIL` | `REOPEN` | `BLOCKED` (distinct from `declared_status`)
   - `ai_audit_findings` — red flag IDs that fired in Step 4 with verdict
   - `action` — `none` | `fixed` | `reopened-frontmatter` | `BUG-NNN.md filed`
   - `linked_bugs` — BUG IDs
3. Verify every completed or claimed-complete task against actual files, public behavior, automated tests, and acceptance criteria. Re-execute the smallest public proof against the current repository state.
4. Assign `qa_verdict`:
   - `PASS`: every material requirement and success criterion has implementation and fresh verification evidence.
   - `PARTIAL`: implementation exists but one or more non-critical requirements, tests, or evidence are missing.
   - `FAIL`: claimed behavior does not work or a critical requirement is absent.
   - `REOPEN`: frontmatter says `status: completed` but the QA verdict is `PARTIAL` or `FAIL`.
   - `BLOCKED`: a concrete prerequisite is missing. Validate every local boundary that does not need the missing dependency and report the blocked live validation separately.

## Step 4: AI Test-Hygiene Scan (RF-1..RF-6)

1. Read `references/ai-implementation-audit.md` in full before scanning the test diff of any task with `declared_status: completed` — it owns the RF-1..RF-6 scanners, the Requirement→Test mapping, and the verdict matrix.
2. Run the scans against the diff since the task baseline (`git log --follow <test_file>`, `git diff <baseline_sha>..HEAD`).
3. Emit the verdict the matrix assigns. RF-1 (skip/only/xit/t.Skip inserted), RF-2 on a P0/P1 criterion (weakened assertion), RF-3 (mock on a dependency the TC declared Integration/E2E), and RF-4 on P0/P1 (unjustified snapshot drift) are automatic `FAIL`.
4. Record findings in the matrix column `ai_audit_findings` and in the per-task block of `audit-report.md`.
5. Apply the Requirement→Test mapping: for every Success Criterion in `task_NN.md` and every linked `_techspec.md` bullet, mark the matching test `covers` / `weak` / `missing`. A checked item or `status: completed` without a `covers` row is an audit failure.

## Step 5: Reopen, File Bugs, Write Memory

1. Mark every incomplete completed task `REOPEN`.
2. **In Compozy mode**, write `memory/qa-execution.md` with the cy-codex-loop canonical sections (`Objective Snapshot`, `Important Decisions`, `Learnings`, `Files / Surfaces`, `Errors / Corrections`, `Ready for Next Run`) **before** flipping any `task_NN.md` frontmatter (memory-precedes-status invariant).
3. Edit the offending `task_NN.md` frontmatter `status:` back to `pending` (or `in_progress` if salvageable). Leave `state.yaml` alone — `update-state.py` owns it, and the next iteration reconciles from frontmatter.
4. File `BUG-<num>.md` under `<audit-output-path>/audit/issues/` using `assets/issue-template.md`, including: the task path (`Reopens task:`), the failed Success Criterion (`Summary:`), the original strict assertion when RF-2 fired (`Root cause:`), the red flag ID and verdict (`Automation Follow-up:`), and any transcript anomaly classification (`Related:`).
5. When the gap is a bounded root-cause fix inside the audit scope, implement it, add regression coverage, and rerun the task proof. Otherwise reopen the task.

## Step 6: Quality Gates Verdict

1. Re-run the canonical verification gate from scratch after the last code change made during the audit.
2. Compile the Quality Gates section of `audit-report.md`, each `PASS` / `FAIL` / `N/A`:
   - Flaky rate <2% in the canonical suite.
   - Zero `FAIL` from the AI test-hygiene scan on P0/P1 tasks.
   - Zero `Critical` / `High` issues open.
   - Coverage delta ≥ baseline (no regression).
   - Zero unresolved `flaky-suspect` on P0 flows.
3. A `FAIL` on any gate blocks an unconditional PASS verdict for the run.

## Step 7: Write the Audit Report

1. Write the report to `<audit-output-path>/audit/audit-report.md` using `assets/audit-report-template.md`, with all mandatory sections:
   - **Claim / Command / Exit code / Verdict** per command executed in Steps 2 and 6.
   - **AUTOMATED COVERAGE** — support detected, harness, canonical command, required flows with classification, specs added or updated.
   - **TASK IMPLEMENTATION AUDIT** — Compozy slug, plan sources, matrix totals, per-task verdicts, reopened/fixed/blocked tasks, links to bugs.
   - **SUITE HEALTH SNAPSHOT** — flaky rate, flaky events, mutation score (when a harness exists), coverage delta vs baseline, blocked count, manual-only count, AI audit findings count.
   - **QUALITY GATES** — PASS/FAIL/N/A per gate.
   - **ISSUES FILED** — total, by severity, with `Reopens task:` annotations.
   - Report each blocked scenario, missing credential, or environment gap with the exact command or prerequisite that stopped execution.
2. In a Compozy slug, a final PASS feeds cy-codex-loop's `verify.last_status=PASS` precondition for Phase E — leave `update-state.py` to cy-codex-loop.
3. Before declaring the audit complete, confirm every item in `references/checklist.md` — it is the exhaustive completion criterion across all steps.
