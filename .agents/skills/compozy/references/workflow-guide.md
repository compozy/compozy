# Workflow Guide

End-to-end walkthrough of the Compozy development pipeline from setup through archive.

## Prerequisites

1. **Install Compozy.** Ensure the `compozy` binary is available in the system PATH.
2. **Run setup.** Execute `compozy setup` to install core skills into target agents plus setup assets from enabled extensions. For a quick start: `compozy setup --all`.
3. **Install optional ideation extension when needed.** To use `/cy-idea-factory`, run `compozy ext install --yes compozy/compozy --remote github --ref <tag> --subdir extensions/cy-idea-factory`, then `compozy ext enable cy-idea-factory`, then `compozy setup` again.
4. **Configure workspace (optional).** Create `.compozy/config.toml` to set default IDE, model, and other preferences. Read `config-reference.md` for all fields.

## Phase 1: Ideation (Optional)

**Skill:** `/cy-idea-factory [feature-idea]`

Use when a raw idea needs structured exploration before committing to a PRD.

Install flow: `compozy ext install --yes compozy/compozy --remote github --ref <tag> --subdir extensions/cy-idea-factory` -> `compozy ext enable cy-idea-factory` -> `compozy setup`

1. Invoke `/cy-idea-factory` inside an agent session with the feature idea.
2. Answer 3-6 clarifying questions (one per message, multiple-choice preferred).
3. The skill runs parallel codebase exploration and web research.
4. A business analyst persona evaluates viability with KPIs and scoring.
5. A council debate (3-5 advisors from the extension-shipped council agents) challenges scope and surfaces risks.
6. A product strategist scans for higher-leverage alternatives.
7. Review and approve the draft idea spec.
8. Output: `.compozy/tasks/<slug>/_idea.md` + ADRs.

**Skip when:** The requirements are already well-understood and a PRD can be written directly.

## Phase 2: Requirements

**Skill:** `/cy-create-prd [feature-name-or-idea] [idea-file]`

1. Invoke `/cy-create-prd` with the feature name. If `_idea.md` exists, it is used as primary context.
2. The skill runs parallel codebase and market research.
3. Answer clarifying questions focused on WHAT and WHY (not HOW).
4. Choose from 2-3 product approaches. An ADR is created for the decision.
5. Review and approve the complete PRD draft.
6. Output: `.compozy/tasks/<slug>/_prd.md` + ADRs.

**Key rule:** The PRD describes user capabilities and business outcomes only. No databases, APIs, frameworks, or architecture.

## Phase 3: Technical Design

**Skill:** `/cy-create-techspec [feature-name]`

1. Invoke `/cy-create-techspec` with the feature name.
2. The skill reads the existing `_prd.md` and explores the codebase architecture.
3. Answer technical clarifying questions.
4. Technical ADRs are created for architecture decisions.
5. Review and approve the TechSpec draft.
6. Output: `.compozy/tasks/<slug>/_techspec.md` + ADRs.

**Contains:** System architecture, data models, core interfaces, API design, development sequencing.

## Phase 4: Task Decomposition

**Skill:** `/cy-create-tasks [feature-name]`

1. Invoke `/cy-create-tasks` with the feature name.
2. The skill loads the PRD and TechSpec, then breaks them into granular tasks.
3. Review the proposed task breakdown.
4. Task files are generated with YAML frontmatter: `status`, `title`, `type`, `complexity`, `dependencies`.
5. Tasks are enriched with codebase patterns and implementation context.
6. Validation runs via `compozy validate-tasks`.
7. Output: `task_01.md` through `task_N.md`, `_tasks.md` master list.

**Task types:** `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`. Custom types can be registered in `.compozy/config.toml` under `[tasks].types`.

## Phase 5: Execution

**Command:** `compozy start --name <slug> --ide <runtime>`

1. Compozy reads task files from `.compozy/tasks/<slug>/` in order, respecting dependencies.
2. For each pending task, Compozy constructs a prompt including the task spec, PRD, TechSpec, ADRs, and workflow memory.
3. The configured ACP runtime executes the task using the `cy-execute-task` skill.
4. Each task: read spec -> implement -> validate with `cy-final-verify` -> update tracking -> optional commit.
5. Workflow memory is maintained across tasks via `cy-workflow-memory`.

**Key flags:**
- `--auto-commit` -- create a local commit after each task completes cleanly.
- `--dry-run` -- generate prompts without running the IDE tool.
- `--include-completed` -- re-process tasks already marked as completed.

**Interactive mode:** When run without flags, an interactive TUI form collects the workflow name, IDE, and options.

## Phase 6: Review

Two paths are available:

### Path A: Manual AI Review

**Skill:** `/cy-review-round [feature-name]`

Invoke inside an agent session. The skill performs a comprehensive code review of the implementation and generates issue files under `.compozy/tasks/<slug>/reviews-NNN/`.

### Path B: External Provider Review

**Command:** `compozy fetch-reviews --provider coderabbit --pr <N> --name <slug>`

Fetches review comments from an external provider (currently CodeRabbit) and writes them as issue markdown files under `reviews-NNN/`.

**Both paths produce:** `_meta.md` (round metadata) and `issue_*.md` files with YAML frontmatter (`status`, `severity`, `file`, `line`, `title`).

## Phase 7: Remediation

**Command:** `compozy fix-reviews --name <slug> --ide <runtime>`

1. Compozy reads issue files from the latest (or specified) review round.
2. For each batch of issues, the configured ACP runtime executes the `cy-fix-reviews` skill.
3. Each issue is triaged (valid/invalid), fixed if valid (in severity order), and verified with `cy-final-verify`.
4. Issue file frontmatter is updated: `pending` -> `valid`/`invalid` -> `resolved`.

**Key flags:**
- `--concurrent <N>` -- process N batches in parallel.
- `--batch-size <N>` -- group N file scopes per batch.
- `--include-resolved` -- re-process already-resolved issues.

**Iterate:** Repeat phases 6-7 until all reviews are clean, then merge.

## Phase 8: Archive

**Command:** `compozy archive --name <slug>`

Moves fully completed workflows from `.compozy/tasks/<slug>/` to `.compozy/tasks/_archived/<timestamp>-<slug>/`.

**Eligibility:** All task files must be completed, and all review round `_meta.md` files must be fully resolved.

## Ad Hoc Execution

**Command:** `compozy exec [prompt]`

Execute a single prompt outside the pipeline workflow.

- **Reusable agents:** `compozy exec --agent council "Evaluate this design"` invokes a named agent.
- **Persistence:** `--persist` saves session artifacts. Resume with `--run-id <id>`.
- **TUI mode:** `--tui` opens an interactive terminal UI.
- **Output formats:** `--format text` (default), `json` (lean JSONL), `raw-json` (full event stream).

## Workflow Memory

The `cy-workflow-memory` skill maintains two tiers of context during task execution:

| File | Purpose |
| --- | --- |
| `.compozy/tasks/<slug>/memory/MEMORY.md` | Shared cross-task memory: architecture decisions, discovered patterns, open risks, handoffs |
| `.compozy/tasks/<slug>/memory/task_NN.md` | Per-task memory: objective snapshot, files touched, errors hit, next steps |

- Memory files are scaffolded before task execution and updated during the run.
- Agents read both files as mandatory context before editing code.
- Promotion from task to shared memory requires: needed by other tasks, durable across runs, and not derivable from existing artifacts.
- Auto-compaction triggers when files exceed size limits.

## Architecture Decision Records

ADRs are created during ideation, PRD, and TechSpec phases to document significant decisions.

- **Location:** `.compozy/tasks/<slug>/adrs/adr-NNN.md` (zero-padded 3-digit numbers).
- **Structure:** Status, Date, Context, Decision, Alternatives Considered, Consequences.
- **Referenced by:** PRDs, TechSpecs, and idea specs include an "Architecture Decision Records" section linking to all ADRs.
