---
name: cy-research-issues
description: Runs a multi-agent research pass over the codebase (and optional competitor sources under .resources/) and converts the findings into an executable Compozy task suite: seven-section analysis documents, a lightweight _techspec.md, and task_NN.md issue files wired into a compozy.tasks/v2 _tasks.md graph, each carrying evidence, competitor references, and unit/integration/e2e test plans so cy-execute-task can run them directly. Use when auditing a subsystem, investigating a bug class, or turning improvement research into a runnable backlog without the interactive PRD/TechSpec ceremony. Do not use for PRD creation (use cy-create-prd), formal TechSpec authoring with ADR review gates (use cy-create-techspec), decomposing an already-approved spec (use cy-create-tasks), or executing the generated tasks (use cy-execute-task).
argument-hint: "[slug] [research-prompt] [auto]"
---

# Research Issues

Turn a research question into an executable Compozy task suite. Pipeline:
**scout → parallel explorer analyses → synthesis → lite techspec → issue-tasks → graph → validate.**
The output directory is `.compozy/tasks/<slug>/` and its task files are directly consumable by
`/compozy` / `cy-execute-task` — same parser contract as `cy-create-tasks` output.

## Required Reading Router

Match the current phase to the row. Read the listed file **in full before** producing that phase's
output. They are not appendices — they are load-bearing. Inline content in this SKILL.md is a
pointer, not a substitute.

| Phase                                                | MUST read                              |
| ---------------------------------------------------- | -------------------------------------- |
| Dispatching or resuming explorer agents (step 3)     | `references/explorer-dispatch.md`      |
| Authoring `_techspec.md` (step 5)                    | `references/techspec-lite-template.md` |
| Generating `task_NN.md` files + `_tasks.md` (step 7) | `references/issue-task-template.md`    |

## Reference Index

- `references/explorer-dispatch.md` — explorer agent contract, the seven-section analysis schema,
  the dispatch prompt template, and the failure/resume protocol for agents killed mid-run.
- `references/techspec-lite-template.md` — the full `_techspec.md` template (sections 1-10 incl.
  AGH Impact Audit and Web/Docs Impact) plus authoring rules (YAGNI, Open Decisions instead of ADR
  ceremony).
- `references/issue-task-template.md` — the complete `task_NN.md` template merging issue DNA with
  the executable contract, the parser rules, the `_tasks.md` graph manifest contract, complexity
  criteria, and test-plan rules.

## Required Inputs

- **Slug** — names the `.compozy/tasks/<slug>/` directory.
- **Research prompt** — the question/goal: bugs to investigate, surfaces to audit, competitor
  behavior to copy. Concrete symptoms ("returning to a live session shows a blank thread") produce
  better analyses than abstract goals.
- Optional: competitor directories under `.resources/<repo>/` to include as reference slices.
- Optional: `auto` — skip the single approval checkpoint (step 6) for unattended runs.

## Hard Rules

- This skill is **analysis + authoring only**. Never modify production code, never execute the
  generated tasks.
- Explorer analyses run through **`compozy exec` with GLM 5.2 via the `pi` harness** (parallel,
  background) — NOT the runtime's native Agent tool. Each `compozy exec` is a self-contained,
  scoped-write dispatch that writes exactly one analysis file. The exact command, the model/harness
  rules (GLM via `pi`; never `opencode`; Opus override for the hardest slices), and the failure
  protocol live in `references/explorer-dispatch.md`.
- Every generated task file MUST satisfy the parser contract and carry a `## Tests` section with
  Unit, Integration, and E2E categories — the contract lives in
  `references/issue-task-template.md`, not here.
- Do not mark the skill complete until `compozy tasks validate --name <slug>` exits 0.

## Workflow

1. **Preflight (lightweight).**
   - Read `.compozy/config.toml`; if it defines `[tasks].types`, use that list for task `type`
     values, otherwise use the defaults (`frontend`, `backend`, `docs`, `test`, `infra`,
     `refactor`, `chore`, `bugfix`).
   - Read `docs/_memory/standing_directives.md` and skim `docs/_memory/lessons/README.md` for
     entries matching the research domain. Cite applicable directives/lessons in the techspec.
   - If `.compozy/tasks/<slug>/analysis/` already contains the analyses for this research
     (resumed run), reuse them and skip steps 2-4.

2. **Scout and slice.**
   - Run fast structural scans (`ls`, `find`, grep/glob) to map the territory — do not deep-read
     files here; that is the explorers' job.
   - Divide the research into 4-8 slices. Standard slicing for an improvement research:
     one slice per competitor repo, one per owned surface layer (UI / client data layer / backend),
     one end-to-end trace per reported bug class, one comparative/mapping slice when copying
     competitor design. Drop slices that do not apply; add domain-specific ones that do.
   - Each slice gets: a number `NN`, a kebab slug, a single question, primary source paths, and
     the target file `analysis/NN_analysis_<slice-slug>.md`.

3. **Dispatch explorers via `compozy exec` (GLM 5.2 / pi).**
   - **STOP. Read `references/explorer-dispatch.md` in full before launching any explorer.** It
     contains the scoped-write contract, the `compozy exec` command and model/harness rules, the
     seven-section schema every analysis must follow, the dispatch prompt template, and the
     failure/resume protocol. Do not improvise dispatch prompts from memory of this file.
   - Write one self-contained prompt file per slice from the template, then background one
     `compozy exec --ide pi --model openrouter/z-ai/glm-5.2 --prompt-file <slice-prompt>` per slice
     (parallel). Wait for every process to exit before synthesis. Escalate a slice to
     `--ide claude --model opus` only when it demands maximal citation precision (see the reference).
   - On any non-zero exit or contract violation, apply the reference's failure protocol (re-dispatch
     the slice with the contract restated).
   - Read each analysis as it lands; note cross-slice contradictions to resolve in synthesis.

4. **Synthesize `analysis/summary.md`.**
   - Lead with root-cause chains for every reported bug (numbered causes, each citing
     `path:line` evidence from the analyses).
   - State the design verdict when competitors were analyzed (what to copy, what to reject, where
     the project should deliberately diverge).
   - End with a table mapping each analysis document to its top value, and a pointer to the
     techspec and task files.

5. **Author `_techspec.md` (lite).**
   - **STOP. Read `references/techspec-lite-template.md` in full before writing the techspec.**
     Fill every section it defines — the AGH Impact Audit and Web/Docs Impact sections are
     mandatory, and contested decisions go to its Open Decisions section instead of blocking on
     interactive rounds.
   - This is the reference document `cy-execute-task` reads for implementation guidance — carry
     root causes, target architecture, delete targets, and sequencing, all evidence-linked.

6. **Checkpoint (skip when `auto`).**
   - Present the proposed task breakdown: titles, `type`, `complexity`, dependency edges, and the
     wave grouping. One message, one approval.
   - Use the runtime's interactive question tool and wait. Incorporate feedback and re-present
     until approved. Do not re-litigate the analyses — only the breakdown is under review.

7. **Generate the task suite.**
   - **STOP. Read `references/issue-task-template.md` in full before writing any task file or
     `_tasks.md`.** It is the single source of truth for the file template, parser rules, graph
     manifest contract, complexity criteria, and test-plan rules. Gist tripwires only:
     - `task_NN.md` naming, frontmatter = `status/title/type/complexity` only, dependencies only
       in `_tasks.md` (`compozy.tasks/v2`, acyclic).
     - Every task carries Problem & Evidence + Expected Behavior (issue DNA) AND Subtasks,
       Deliverables, Tests (Unit/Integration/E2E), Success Criteria (executable contract).
     - Test cases name specific inputs/conditions and the canonical owning suite.
   - Derive test plans from the analyses; keep tasks independently implementable — if two tasks
     are tightly coupled, merge them or extract the shared piece into a dependency task.

8. **Validate.**
   - Run `compozy tasks validate --name <slug>` (read-only check). Fix reported issues and re-run
     until it exits 0.

9. **Report.**
   - Print the deliverable tree, the suggested execution order, and the exact follow-up command
     for execution (e.g., `/compozy <slug>`). State explicitly that no production code changed.

## Anti-Patterns

- **Analysis without line numbers.** Every claim in analyses, techspec, and tasks cites
  `path:line`. An uncited claim is a hallucination gap for the executor.
- **Mega-tasks.** >7 files or >7 subtasks → split with explicit graph edges.
- **TechSpec duplication.** Tasks reference techspec sections by name; they never copy its
  diagrams/interfaces.
- **Silent scope caps.** If slices, analyses, or issues were dropped for budget, say so in the
  report — never imply full coverage.

## Error Handling

- Explorer failures (non-zero `compozy exec` exit or stalled slice) → follow
  `references/explorer-dispatch.md` §Failure recovery (re-dispatch the slice; escalate to Opus after
  two GLM failures). If every slice stalls with zero output, confirm dispatch used `--ide pi` (not
  `opencode`) and that `~/.pi/agent/auth.json` holds the OpenRouter key.
- Analyses contradict each other → resolve in synthesis with evidence, or mark `UNCONFIRMED` with
  the verification step the executor must run first.
- `compozy tasks validate` keeps failing on the graph → re-check the manifest against
  `references/issue-task-template.md` §Graph Manifest before touching task content.
- User rejects the breakdown at the checkpoint → incorporate all feedback before re-presenting;
  never write task files from an unapproved breakdown (unless `auto`).
