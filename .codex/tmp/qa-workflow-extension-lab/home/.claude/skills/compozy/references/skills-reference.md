# Bundled Skills Reference

Detailed catalog of all bundled Compozy skills, their inputs, outputs, and pipeline position.

---

## cy-idea-factory

**Trigger:** `/cy-idea-factory [feature-idea]`

Install first: `compozy ext install --yes compozy/compozy --remote github --ref <tag> --subdir extensions/cy-idea-factory` -> `compozy ext enable cy-idea-factory` -> `compozy setup`

Expands a raw feature idea into a structured, research-backed specification through interactive brainstorming, web research, business analysis, and multi-advisor council debate.

- **Inputs:** Feature idea or problem description. Optional existing `_idea.md` for update mode.
- **Outputs:** `.compozy/tasks/<slug>/_idea.md`, ADRs in `adrs/`.
- **Pipeline position:** Optional first step. Feeds into `cy-create-prd`.
- **Process:** Clarifying questions -> parallel codebase + web research -> business viability analysis -> council debate -> opportunity scan -> draft -> user approval.
- **Use when:** The user has a raw idea and wants to explore viability before committing to a PRD.
- **Do not use for:** PRD creation, technical specifications, task breakdown, or code implementation.

---

## cy-create-prd

**Trigger:** `/cy-create-prd [feature-name-or-idea] [idea-file]`

Creates a business-focused Product Requirements Document through structured brainstorming with parallel codebase and web research.

- **Inputs:** Feature name or idea. Optional existing `_idea.md` or `_prd.md` for update mode.
- **Outputs:** `.compozy/tasks/<slug>/_prd.md`, ADRs in `adrs/`.
- **Pipeline position:** After ideation (optional). Feeds into `cy-create-techspec`.
- **Process:** Context discovery (codebase + web) -> clarifying questions -> 2-3 product approaches -> ADR for chosen approach -> draft PRD -> user approval.
- **Use when:** Starting a new feature or product, building or updating a PRD.
- **Do not use for:** Technical specifications, task breakdowns, or code implementation.

---

## cy-create-techspec

**Trigger:** `/cy-create-techspec [feature-name]`

Translates PRD business requirements into a technical implementation design.

- **Inputs:** Existing `_prd.md` from the previous stage.
- **Outputs:** `.compozy/tasks/<slug>/_techspec.md`, ADRs in `adrs/`.
- **Pipeline position:** After PRD. Feeds into `cy-create-tasks`.
- **Process:** Codebase architecture exploration -> technical questions -> technical ADRs -> TechSpec draft -> user approval.
- **Use when:** A PRD exists and needs a technical implementation plan.
- **Do not use for:** PRD creation, task execution, or code implementation.

---

## cy-create-tasks

**Trigger:** `/cy-create-tasks [feature-name]`

Decomposes PRDs and TechSpecs into detailed, independently implementable task files with codebase-informed enrichment.

- **Inputs:** Existing `_prd.md` and `_techspec.md`.
- **Outputs:** Individual task files (`task_01.md`, `task_02.md`, etc.), `_tasks.md` master list.
- **Pipeline position:** After TechSpec. Feeds into `compozy tasks run`.
- **Process:** Load PRD+TechSpec context -> break into granular tasks -> user approval -> generate task files -> enrich with codebase patterns -> validate with `compozy tasks validate`.
- **Task metadata:** Each task has YAML frontmatter with `status` (pending/in_progress/completed), `title`, `type`, `complexity`, and `dependencies`.
- **Use when:** A PRD and TechSpec exist and need to be broken into executable tasks.
- **Do not use for:** Execution, review, or code implementation.

---

## cy-execute-task

**Trigger:** Internal (called by `compozy tasks run`). Do not invoke directly.

Executes one PRD task end-to-end using the provided task file, PRD directory, and tracking file paths.

- **Inputs:** Task specification, PRD directory path, task file path, master tasks file path, auto-commit mode. Optional workflow memory paths.
- **Outputs:** Implemented code changes, updated task tracking files, optional commit.
- **Pipeline position:** Called by `compozy tasks run` for each task in sequence.
- **Process:** Ground in PRD/TechSpec context -> build execution checklist -> implement -> validate with `cy-final-verify` -> update tracking -> optional commit.
- **Use when:** Invoked internally by the execution pipeline.
- **Do not use for:** Direct invocation, PR review batches, or standalone verification.

---

## cy-review-round

**Trigger:** `/cy-review-round [feature-name]`

Performs a comprehensive code review of a PRD implementation and generates review issue files.

- **Inputs:** Feature name identifying the workflow under `.compozy/tasks/<slug>/`.
- **Outputs:** Review round directory `reviews-NNN/` with `issue_*.md` files containing round metadata in YAML frontmatter.
- **Pipeline position:** After execution. Outputs feed into `cy-fix-reviews`.
- **Use when:** Reviewing implemented PRD tasks without an external review provider.
- **Do not use for:** Fetching external reviews (use `compozy reviews fetch`), fixing issues (use `compozy reviews fix`).

---

## cy-fix-reviews

**Trigger:** Internal (called by `compozy reviews fix`). Do not invoke directly.

Executes provider-agnostic PR review remediation using existing review round files.

- **Inputs:** Scoped issue files from the review round and their YAML frontmatter.
- **Outputs:** Updated issue files with triage and status, code fixes, verification evidence.
- **Pipeline position:** Called by `compozy reviews fix`. Operates on output from `cy-review-round` or `compozy reviews fetch`.
- **Process:** Read round context -> triage issues (valid/invalid) -> fix valid issues in severity order -> verify with `cy-final-verify` -> close out issue files.
- **Use when:** Invoked internally by the review remediation pipeline.
- **Do not use for:** Fetching reviews, PRD task execution, or generic coding.

---

## cy-final-verify

**Trigger:** `/cy-final-verify`

Enforces fresh verification evidence before any completion, fix, or passing claim, and before commits or PR creation.

- **Inputs:** None. Operates on the current workspace state.
- **Outputs:** Verification Report with claim, command, exit code, output summary, and verdict.
- **Pipeline position:** Used at the end of `cy-execute-task`, `cy-fix-reviews`, and any completion claim.
- **Core principle:** Evidence before claims, always. No completion claims without fresh verification evidence.
- **Process:** Identify verification command -> execute full command -> read complete output -> verify exit code and counts -> report with evidence.
- **Use when:** About to report success, hand off work, commit code, or create a PR.
- **Do not use for:** Early planning, brainstorming, or tasks not yet at a verification step.

---

## cy-workflow-memory

**Trigger:** Internal (called by `cy-execute-task`). Do not invoke directly.

Maintains workflow-scoped task memory for Compozy runs using files under `.compozy/tasks/<name>/memory/`.

- **Inputs:** Workflow memory directory path, shared memory file path, current task memory file path.
- **Outputs:** Updated `MEMORY.md` (shared) and per-task memory files.
- **Pipeline position:** Used during task execution to maintain cross-task context.
- **Two-tier memory:** Shared workflow memory (`MEMORY.md`) for cross-task decisions and risks. Per-task memory for task-local operational details.
- **Promotion test:** Items promoted from task to shared memory only when needed by other tasks, durable across runs, and not derivable from existing artifacts.
- **Use when:** Task execution requires reading or updating workflow memory.
- **Do not use for:** PR review remediation, global user preferences, or event-log summarization.

---

## compozy

**Trigger:** `/compozy`

This skill. Explains Compozy capabilities, CLI commands, core workflow skills, optional extension skills, configuration, artifact structure, reusable agents, and extensions.

- **Inputs:** None.
- **Outputs:** Reference information presented to the agent.
- **Pipeline position:** Standalone reference, not part of the pipeline.
- **Use when:** The user asks how to use Compozy, what commands are available, or how the workflow works.
- **Do not use for:** Executing any workflow step. Use the specific `cy-` skills instead.
