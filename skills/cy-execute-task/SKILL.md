---
name: cy-execute-task
description: Executes one PRD task end-to-end uninterrupted — resolves spec conflicts autonomously, implements, validates, and updates tracking without pausing for questions. Use when a prompt includes a task specification that must be implemented, validated, and reflected in task tracking files. Do not use for PR review batches, generic coding tasks without a PRD task file, or standalone verification-only work.
---

# Execute PRD Task

Execute one PRD task from exploration through tracking updates — **uninterrupted**.

## Uninterrupted execution

Compozy runs tasks in a loop. Pausing for clarification breaks the loop.

- Never ask the user a question, present option menus, or wait for confirmation while executing a task.
- Never stop because TechSpec, ADRs, task file, or sibling catalogs disagree — resolve the conflict with the Authority and Contract Precedence rules below, record the chosen interpretation, and continue.
- Ambiguity is a decision to make, not a reason to halt. Prefer the interpretation that is most consistent with machine-checkable contracts and assigned test IDs, then implement it.
- Surface decisions only as brief notes in the checklist, workflow memory (when provided), or follow-up notes — never as a blocking prompt.

## Required Inputs

- Task specification markdown.
- PRD directory path.
- Task file path.
- `_tasks.md` task graph manifest path.
- Auto-commit mode.
- Optional workflow memory directory path.
- Optional shared workflow memory path.
- Optional current task memory path.

## Workflow

1. Ground in repository and PRD context.
   - Read the provided task specification.
   - Read the repository guidance files named by the caller.
   - Run the Spec Corpus Survey (section below) with a native read-only subagent. This is mandatory before any edit — the task file is never the whole contract.
   - Read the PRD documents under the provided directory, especially `_techspec.md`, `_tasks.md`, and the contract catalogs `_user_stories.md` and `_tests.md` when present.
   - Read ADRs from the `adrs/` subdirectory of the PRD directory to understand the architectural decision context for this task.
   - Read in full every sibling artifact the survey flags as contract-bearing for this task, and the `analysis/` summaries for decision context.
   - After reading all sources, check for conflicts between the task specification, techspec, ADRs, and the contract-bearing siblings. When sources contradict, apply Authority and Contract Precedence, pick one canonical interpretation, note it in one checklist line, and proceed to step 2 — do not pause.
   - If the caller provides workflow memory paths, use the installed `cy-workflow-memory` skill before editing code.
   - Reconcile the current workspace state before new edits.

2. Build the execution checklist.
   - Extract deliverables, acceptance criteria, and every explicit `Validation`, `Test Plan`, or `Testing` item into a numbered working checklist.
   - Add one checklist line per test-case ID assigned in the task's `## Tests` section, and implement each case as `_tests.md` specifies it — the assigned IDs are part of the deliverable, not a suggestion.
   - Add one checklist line per concrete contract fact extracted from the contract-bearing spec artifacts (input names/types/defaults/required flags, command and route names, node topologies, declared behaviors) — parity with these facts is part of the deliverable.
   - Include any conflict-resolution decisions from step 1 as checklist lines so the chosen interpretation stays visible during implementation.
   - Print the full checklist before starting implementation so it is visible and trackable.
   - Capture the concrete pre-change signal that proves the task is not finished yet.
   - Use this checklist as a gate: mark each item done as evidence is produced during implementation, and do not proceed to validation until every checklist item has been addressed.

3. Implement the task.
   - Keep scope tight to the task specification and the resolved contract interpretation.
   - Follow repository patterns and real dependency APIs.
   - Record meaningful out-of-scope work as follow-up notes instead of silently expanding the task.

4. Validate and self-review.
   - Run every test and validation command listed in the task specification — not just the repository-wide verification.
   - Use the installed `cy-final-verify` skill. This step is mandatory regardless of auto-commit mode — always verify before claiming completion.
   - Check the finished deliverable field by field against every contract-bearing spec artifact identified by the survey (cy-final-verify "Spec Contract Parity"), using the resolved interpretation from step 1 when sources had disagreed. A mismatch against that resolved contract fails completion — fix the deliverable and re-verify; do not reinterpret the contract to match what was built, and do not pause to ask.
   - Perform a self-review after verification and resolve every blocking issue before proceeding.

5. Update task tracking.
   - If workflow memory paths were provided, update the memory files first — record decisions (including conflict resolutions), learnings, and touched surfaces before updating tracking status.
   - Use the caller-provided task file path and task graph manifest path.
   - Mark subtasks complete only when the implementation and evidence are actually complete.
   - Change task status to completed only after clean verification and self-review.
   - Read `references/tracking-checklist.md` when applying status, checklist, or commit updates.
   - Do not edit `_tasks.md` during normal completion tracking; it owns graph topology, not per-task status.
   - Sequence: memory update (if applicable) -> task file checkboxes -> task status -> commit (if applicable).

6. Handle commit behavior.
   - If auto-commit is enabled, create one local commit after clean verification, self-review, and tracking updates.
   - If auto-commit is disabled, leave the diff ready for manual review and commit.
   - Never push automatically.

## Spec Corpus Survey (mandatory before any edit)

Task files paraphrase the spec, and paraphrases drift. Real incident: a task was implemented straight from its task file while the spec directory contained the canonical, complete definition of the deliverable (`_examples.md`) and the QA input contract (`_qa.md`); neither was read, seven review rounds validated engineering quality only, and the shipped result contradicted the product contract wholesale. This survey exists so that can never repeat.

1. Dispatch the agent's NATIVE read-only subagent facility (e.g. `Explore` in Claude Code, or the closest scoped read-only subagent the current harness offers) to inventory the ENTIRE spec directory — every file, not a subset: all `_*.md` siblings (`_prd.md`, `_user_stories.md`, `_techspec.md`, `_tests.md`, `_tasks.md`, `_examples.md`, `_qa.md`, and any others present), loose siblings such as `requirements.md` and `product-ux.md`, plus the `adrs/`, `analysis/` (including `summary*.md`), `handoffs/`, and `memory/` subdirectories.
2. Require the subagent to return, per file: a one-line description and a CONTRACT-BEARING verdict — does this file pin concrete facts about this task's deliverables (canonical example documents, input/schema tables, parity maps, test contracts, CLI/UX surfaces)?
3. If the harness has no subagent facility, perform the same inventory inline before any edit. Skipping the survey because the task file "looks complete" is forbidden — looking complete is exactly how the incident happened.
4. Read in full, on the main thread, every contract-bearing file for this task. Read the `analysis/` summaries (at minimum the newest `summary*.md`) for the decision context behind the spec.

## Authority and Contract Precedence

Resolve contradictions autonomously with this ladder (highest wins). Record the pick; continue.

1. Machine-checkable TechSpec constraints (schemas, SQL `CHECK`s, typed tables, enumerated states) beat conflicting prose in the same TechSpec.
2. Contract-bearing sibling catalogs that pin concrete form for this task (`_tests.md` assigned cases, `_examples.md`, `_qa.md` input tables, `_user_stories.md` acceptance criteria, parity maps) beat a task-file paraphrase of the same fact.
3. When TechSpec and a sibling catalog disagree on concrete form: prefer the sibling catalog for facts that catalog owns (assigned test IDs, canonical examples, QA inputs); prefer the TechSpec machine-checkable constraint for storage/schema/state-machine facts.
4. ADRs beat informal `analysis/` notes when they address the same decision.
5. Among remaining ties, prefer the interpretation that satisfies the most assigned `_tests.md` cases and remains implementable against the winning schema/state constraints.
6. A task-file paraphrase never overrides a higher rung — implementing the paraphrase against a higher-rung contract is the error.

Authority for WHAT to build remains the task file + PRD + TechSpec + ADRs, read through this ladder.

The existing runtime shape is never the contract. If the current code cannot express what the resolved contract requires, extend or adapt the runtime within task scope; if that is truly out of scope, implement the closest faithful solution, record the gap as a follow-up note, and continue — never pause to ask, and never mold the deliverable to whatever the runtime happens to support today without recording the gap.

## Error Handling

- If the pre-change signal cannot be reproduced directly, capture the strongest available baseline signal, note the limitation, and continue.
- If validation fails, keep the task status unchanged, fix the failure, and re-verify until clean — do not ask whether to proceed.
- If tracking files are missing, stop and report the missing path before marking completion (infrastructure failure, not a design conflict).
